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
	"github.com/k0sproject/rig/v2/cmd"
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

// Before runs "before install" hooks for workers
func (p *InstallWorkers) Before() error {
	return p.runHooks(context.Background(), "install", "before", p.hosts...)
}

// After runs "after install" hooks for workers
func (p *InstallWorkers) After() error {
	// Run per-host "after install" hooks via the common helper
	if err := p.runHooks(context.Background(), "install", "after", p.hosts...); err != nil {
		return err
	}

	// Invalidate any created join tokens and overwrite token files
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
			return p.leader.Sudo().Exec(p.leader.Configurer.K0sCmdf("token invalidate --data-dir=%s %s", p.leader.K0sDataDir(), h.Metadata.K0sTokenData.ID))
		})
		if err != nil {
			log.Warnf("%s: failed to invalidate worker join token: %v", p.leader, err)
		}
		_ = p.Wet(h, "overwrite k0s join token file", func() error {
			content := "# overwritten by k0sctl after join\n"
			if p.Config.Spec.K0s.Version.Equal(workerTokenWorkaroundVersion) {
				log.Debugf("%s: configured k0s version is %s, using workaround content for join token file", h, p.Config.Spec.K0s.Version)
				dummyToken, err := buildDummyJoinToken()
				if err != nil {
					return fmt.Errorf("build dummy join token: %w", err)
				}
				content = dummyToken
			}
			if err := h.Sudo().FS().WriteFile(h.K0sJoinTokenPath(), []byte(content), 0o600); err != nil {
				log.Warnf("%s: failed to overwrite the join token file at %s", h, h.K0sJoinTokenPath())
			}
			return nil
		})
	}
	return nil
}

// CleanUp attempts to clean up any changes after a failed install
func (p *InstallWorkers) CleanUp() {
	_ = p.hosts.Filter(func(h *cluster.Host) bool {
		return !h.Metadata.Ready
	}).ParallelEach(context.Background(), func(_ context.Context, h *cluster.Host) error {
		log.Infof("%s: cleaning up", h)
		if len(h.Environment) > 0 {
			if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to get service %s: %v", h, h.K0sServiceName(), err)
			} else if err := svc.SetEnvironment(context.Background(), map[string]string{}); err != nil {
				log.Warnf("%s: failed to clean up service environment: %v", h, err)
			}
		}
		if h.Metadata.K0sInstalled && p.IsWet() {
			if err := h.Sudo().Exec(h.K0sResetCommand()); err != nil {
				log.Warnf("%s: k0s reset failed", h)
			}
		}
		return nil
	})
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

	return p.parallelDo(ctx, p.hosts, func(_ context.Context, h *cluster.Host) error {
		tokenPath := h.K0sJoinTokenPath()
		err := p.Wet(h, fmt.Sprintf("write k0s join token to %s", tokenPath), func() error {
			if err := h.Sudo().FS().MkdirAll(h.FS().Dir(tokenPath), 0o700); err != nil {
				log.Warnf("%s: failed to create k0s config dir %s: %v", h, h.K0sDataDir(), err)
			}
			log.Infof("%s: writing join token to %s", h, tokenPath)
			return h.Sudo().FS().WriteFile(tokenPath, []byte(h.Metadata.K0sTokenData.Token), 0o600)
		})
		if err != nil {
			return err
		}

		err = p.Wet(h, "validate api connection to control plane", func() error {
			log.Infof("%s: validating api connection to %s using join token", h, h.Metadata.K0sTokenData.URL)
			tempfile, err := h.FS().CreateTemp("", "")
			if err != nil {
				return fmt.Errorf("failed to create temp file for kubeconfig: %w", err)
			}
			log.Debugf("%s: temp file path: %q", h, tempfile)
			tempfileHostPath := h.FS().NativePath(tempfile)
			log.Debugf("%s: writing temp kubeconfig file %q", h, tempfileHostPath)
			if err := h.Sudo().FS().WriteFile(tempfile, h.Metadata.K0sTokenData.Kubeconfig, 0o600); err != nil {
				return fmt.Errorf("failed to write temp kubeconfig file: %w", err)
			}

			defer func() {
				if err := h.Sudo().FS().Remove(tempfile); err != nil {
					log.Warnf("%s: failed to delete temp kubeconfig file %s: %v", h, tempfileHostPath, err)
				}
			}()

			err = retry.WithDefaultTimeout(ctx, func(_ context.Context) error {
				err := h.Sudo().Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get --raw=/version --kubeconfig=%s", h.FS().ShellQuote(tempfileHostPath)))
				if err != nil {
					return fmt.Errorf("failed to connect to kubernetes api using the join token - check networking: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("connectivity check failed: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		svc, svcErr := h.Sudo().Service(h.K0sServiceName())
		if svcErr != nil {
			return fmt.Errorf("get service %s: %w", h.K0sServiceName(), svcErr)
		}
		if svc.IsRunning(ctx) {
			err := p.Wet(h, "stop existing k0s service", func() error {
				log.Infof("%s: stopping service", h)
				return svc.Stop(ctx)
			})
			if err != nil {
				return err
			}
		}
		if sp, err := svc.ScriptPath(ctx); err == nil && sp != "" {
			if h.FS().FileExist(sp) {
				err := p.Wet(h, "remove existing k0s service file", func() error {
					return h.Sudo().FS().Remove(sp)
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

		installCmd, err := h.K0sInstallCommand()
		if err != nil {
			return err
		}
		err = p.Wet(h, fmt.Sprintf("install k0s worker with `%s`", strings.ReplaceAll(installCmd, h.K0sInstallLocation(), "k0s")), func() error {
			sudo := h.Sudo()
			if h.IsWindows() {
				return sudo.Exec(installCmd, cmd.AllowWinStderr())
			}
			return sudo.Exec(installCmd)
		})
		if err != nil {
			return err
		}

		h.Metadata.K0sInstalled = true

		if len(h.Environment) > 0 {
			err := p.Wet(h, "update service environment variables", func() error {
				log.Infof("%s: updating service environment", h)
				return svc.SetEnvironment(ctx, h.Environment)
			})
			if err != nil {
				return err
			}
		}

		if p.IsWet() {
			log.Infof("%s: starting service", h)
			if err := svc.Start(ctx); err != nil {
				return err
			}
		}

		if NoWait {
			log.Debugf("%s: not waiting because --no-wait given", h)
			h.Metadata.Ready = true
		} else {
			log.Infof("%s: waiting for node to become ready", h)

			if p.IsWet() {
				if err := retry.WithDefaultTimeout(ctx, node.KubeNodeReadyFunc(h)); err != nil {
					return err
				}
				h.Metadata.Ready = true
			}
		}

		h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version

		return nil
	})
}
