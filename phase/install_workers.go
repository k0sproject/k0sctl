package phase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// InstallWorkers installs k0s on worker hosts and joins them to the cluster
type InstallWorkers struct {
	GenericPhase
	hosts  cluster.Hosts
	leader *cluster.Host
}

// Title for the phase
func (p *InstallWorkers) Title() string {
	return "Install workers"
}

// Prepare the phase
func (p *InstallWorkers) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Workers().Filter(func(h *cluster.Host) bool {
		return !h.Reset && !h.Metadata.NeedsUpgrade && (h.Metadata.K0sRunningVersion == nil || !h.Metadata.Ready)
	})
	p.leader = p.Config.Spec.K0sLeader()

	return nil
}

// ShouldRun is true when there are workers
func (p *InstallWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Before runs "before apply" hooks
func (p *InstallWorkers) Before() error {
	err := p.hosts.ParallelEach(context.Background(), func(_ context.Context, h *cluster.Host) error {
		if !p.IsWet() && h.HasHooks("apply", "before") {
			p.DryMsg(h, "run before apply hooks")
			return nil
		}

		if err := h.RunHooks("apply", "before"); err != nil {
			return fmt.Errorf("failed to run before apply hooks: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return p.hosts.ParallelEach(context.Background(), func(_ context.Context, h *cluster.Host) error {
		if !p.IsWet() && h.HasHooks("install", "before") {
			p.DryMsg(h, "run before install hooks")
			return nil
		}

		if err := h.RunHooks("install", "before"); err != nil {
			return fmt.Errorf("failed to run before install hooks: %w", err)
		}

		return nil
	})
}

// After runs "after apply" hooks
func (p *InstallWorkers) After() error {
	err := p.hosts.ParallelEach(context.Background(), func(_ context.Context, h *cluster.Host) error {
		if !p.IsWet() && h.HasHooks("apply", "after") {
			p.DryMsg(h, "run after apply hooks")
			return nil
		}

		if err := h.RunHooks("apply", "after"); err != nil {
			return fmt.Errorf("failed to run after apply hooks: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return p.hosts.ParallelEach(context.Background(), func(_ context.Context, h *cluster.Host) error {
		if !p.IsWet() && h.HasHooks("install", "after") {
			p.DryMsg(h, "run after install hooks")
			return nil
		}

		if err := h.RunHooks("install", "after"); err != nil {
			return fmt.Errorf("failed to run after install hooks: %w", err)
		}

		return nil
	})
}

// CleanUp attempts to clean up any changes after a failed install
func (p *InstallWorkers) CleanUp() {
	_ = p.AfterHook()
	_ = p.hosts.Filter(func(h *cluster.Host) bool {
		return !h.Metadata.Ready
	}).ParallelEach(context.Background(), func(_ context.Context, h *cluster.Host) error {
		log.Infof("%s: cleaning up", h)
		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %v", h, err)
			}
		}
		if h.Metadata.K0sInstalled && p.IsWet() {
			if err := h.Exec(h.Configurer.K0sCmdf("reset --data-dir=%s", h.K0sDataDir()), exec.Sudo(h)); err != nil {
				log.Warnf("%s: k0s reset failed", h)
			}
		}
		return nil
	})
}

func (p *InstallWorkers) AfterHook() error {
	if NoWait {
		for _, h := range p.hosts {
			if h.Metadata.K0sTokenData.Token != "" {
				log.Warnf("%s: --no-wait given, created join tokens will remain valid for 10 minutes", p.leader)
				break
			}
		}
		return nil
	}
	for i, h := range p.hosts {
		h.Metadata.K0sTokenData.Token = ""
		if h.Metadata.K0sTokenData.ID == "" {
			continue
		}
		err := p.Wet(p.leader, fmt.Sprintf("invalidate k0s join token for worker %s", h), func() error {
			log.Debugf("%s: invalidating join token for worker %d", p.leader, i+1)
			return p.leader.Exec(p.leader.Configurer.K0sCmdf("token invalidate --data-dir=%s %s", p.leader.K0sDataDir(), h.Metadata.K0sTokenData.ID), exec.Sudo(p.leader))
		})
		if err != nil {
			log.Warnf("%s: failed to invalidate worker join token: %v", p.leader, err)
		}
		_ = p.Wet(h, "overwrite k0s join token file", func() error {
			if err := h.Configurer.WriteFile(h, h.K0sJoinTokenPath(), "# overwritten by k0sctl after join\n", "0600"); err != nil {
				log.Warnf("%s: failed to overwrite the join token file at %s", h, h.K0sJoinTokenPath())
			}
			return nil
		})
	}
	return nil
}

// Run the phase
func (p *InstallWorkers) Run(ctx context.Context) error {
	for i, h := range p.hosts {
		log.Infof("%s: generating a join token for worker %d", p.leader, i+1)
		err := p.Wet(p.leader, fmt.Sprintf("generate a k0s join token for worker %s", h), func() error {
			t, err := p.Config.Spec.K0s.GenerateToken(
				ctx,
				p.leader,
				"worker",
				time.Duration(10*time.Minute),
			)
			if err != nil {
				return err
			}

			td, err := cluster.ParseToken(t)
			if err != nil {
				return fmt.Errorf("parse k0s token: %w", err)
			}

			h.Metadata.K0sTokenData = td

			return nil
		}, func() error {
			h.Metadata.K0sTokenData.ID = "dry-run"
			h.Metadata.K0sTokenData.URL = p.Config.Spec.KubeAPIURL()
			return nil
		})
		if err != nil {
			return err
		}
	}

	err := p.parallelDo(ctx, p.hosts, func(_ context.Context, h *cluster.Host) error {
		if p.IsWet() || !p.leader.Metadata.DryRunFakeLeader {
			log.Infof("%s: validating api connection to %s using join token", h, h.Metadata.K0sTokenData.URL)
			err := retry.AdaptiveTimeout(ctx, 30*time.Second, func(_ context.Context) error {
				err := h.Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get --raw='/version' --kubeconfig=/dev/stdin"), exec.Sudo(h), exec.Stdin(string(h.Metadata.K0sTokenData.Kubeconfig)))
				if err != nil {
					return fmt.Errorf("failed to connect to kubernetes api using the join token - check networking: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("connectivity check failed: %w", err)
			}
		} else {
			log.Warnf("%s: dry-run: skipping api connection validation because cluster is not actually running", h)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return p.parallelDo(ctx, p.hosts, func(_ context.Context, h *cluster.Host) error {
		tokenPath := h.K0sJoinTokenPath()
		err := p.Wet(h, fmt.Sprintf("write k0s join token to %s", tokenPath), func() error {
			log.Infof("%s: writing join token to %s", h, tokenPath)
			return h.Configurer.WriteFile(h, tokenPath, h.Metadata.K0sTokenData.Token, "0600")
		})
		if err != nil {
			return err
		}

		if sp, err := h.Configurer.ServiceScriptPath(h, h.K0sServiceName()); err == nil {
			if h.Configurer.ServiceIsRunning(h, h.K0sServiceName()) {
				err := p.Wet(h, "stop existing k0s service", func() error {
					log.Infof("%s: stopping service", h)
					return h.Configurer.StopService(h, h.K0sServiceName())
				})
				if err != nil {
					return err
				}
			}
			if h.Configurer.FileExist(h, sp) {
				err := p.Wet(h, "remove existing k0s service file", func() error {
					return h.Configurer.DeleteFile(h, sp)
				})
				if err != nil {
					return err
				}
			}
		}

		log.Infof("%s: installing k0s worker", h)
		if Force {
			log.Warnf("%s: --force given, using k0s install with --force", h)
			h.InstallFlags.AddOrReplace("--force=true")
		}

		cmd, err := h.K0sInstallCommand()
		if err != nil {
			return err
		}
		err = p.Wet(h, fmt.Sprintf("install k0s worker with `%s`", strings.ReplaceAll(cmd, h.Configurer.K0sBinaryPath(), "k0s")), func() error {
			return h.Exec(cmd, exec.Sudo(h))
		})
		if err != nil {
			return err
		}

		h.Metadata.K0sInstalled = true

		if len(h.Environment) > 0 {
			err := p.Wet(h, "update service environment variables", func() error {
				log.Infof("%s: updating service environment", h)
				return h.Configurer.UpdateServiceEnvironment(h, h.K0sServiceName(), h.Environment)
			})
			if err != nil {
				return err
			}
		}

		if p.IsWet() {
			log.Infof("%s: starting service", h)
			if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
				return err
			}
		}

		if NoWait {
			log.Debugf("%s: not waiting because --no-wait given", h)
			h.Metadata.Ready = true
		} else {
			log.Infof("%s: waiting for node to become ready", h)

			if p.IsWet() {
				if err := retry.AdaptiveTimeout(ctx, retry.DefaultTimeout, node.KubeNodeReadyFunc(h)); err != nil {
					return err
				}
				h.Metadata.Ready = true
			}
		}

		h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version

		return nil
	})
}
