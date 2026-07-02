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
				p.leader.Log().Warnf("--no-wait given, created join tokens will remain valid for 10 minutes")
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
			p.leader.Log().Debugf("invalidating join token for worker %d", i+1)
			return p.leader.Sudo().Exec(p.leader.Configurer.K0sCmdf("token invalidate --data-dir=%s %s", p.leader.K0sDataDir(), h.Metadata.K0sTokenData.ID))
		})
		if err != nil {
			p.leader.Log().Warnf("failed to invalidate worker join token: %v", err)
		}
		_ = p.Wet(h, "overwrite k0s join token file", func() error {
			content := "# overwritten by k0sctl after join\n"
			if p.Config.Spec.K0s.Version.Equal(workerTokenWorkaroundVersion) {
				h.Log().Debugf("configured k0s version is %s, using workaround content for join token file", p.Config.Spec.K0s.Version)
				dummyToken, err := buildDummyJoinToken()
				if err != nil {
					return fmt.Errorf("build dummy join token: %w", err)
				}
				content = dummyToken
			}
			if err := h.Sudo().FS().WriteFile(h.K0sJoinTokenPath(), []byte(content), 0o600); err != nil {
				h.Log().Warnf("failed to overwrite the join token file at %s", h.K0sJoinTokenPath())
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
		h.Log().Infof("cleaning up")
		if len(h.Environment) > 0 {
			if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
				h.Log().Warnf("failed to get service %s: %v", h.K0sServiceName(), err)
			} else if err := svc.SetEnvironment(context.Background(), map[string]string{}); err != nil {
				h.Log().Warnf("failed to clean up service environment: %v", err)
			}
		}
		if h.Metadata.K0sInstalled && p.IsWet() {
			if err := h.Sudo().Exec(h.K0sResetCommand()); err != nil {
				h.Log().Warnf("k0s reset failed")
			}
		}
		return nil
	})
}

// Run the phase
func (p *InstallWorkers) Run(ctx context.Context) error {
	for i, h := range p.hosts {
		p.leader.Log().Infof("generating a join token for worker %d", i+1)
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
				h.Log().Warnf("failed to create k0s config dir %s: %v", h.K0sDataDir(), err)
			}
			h.Log().Infof("writing join token to %s", tokenPath)
			return h.Sudo().FS().WriteFile(tokenPath, []byte(h.Metadata.K0sTokenData.Token), 0o600)
		})
		if err != nil {
			return err
		}

		err = p.Wet(h, "validate api connection to control plane", func() error {
			h.Log().Infof("validating api connection to %s using join token", h.Metadata.K0sTokenData.URL)
			tempfile, err := h.FS().CreateTemp("", "")
			if err != nil {
				return fmt.Errorf("failed to create temp file for kubeconfig: %w", err)
			}
			h.Log().Debugf("temp file path: %q", tempfile)
			tempfileHostPath := h.FS().NativePath(tempfile)
			h.Log().Debugf("writing temp kubeconfig file %q", tempfileHostPath)
			if err := h.Sudo().FS().WriteFile(tempfile, h.Metadata.K0sTokenData.Kubeconfig, 0o600); err != nil {
				return fmt.Errorf("failed to write temp kubeconfig file: %w", err)
			}

			defer func() {
				if err := h.Sudo().FS().Remove(tempfile); err != nil {
					h.Log().Warnf("failed to delete temp kubeconfig file %s: %v", tempfileHostPath, err)
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
				h.Log().Infof("stopping service")
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

		h.Log().Infof("installing k0s worker")
		if Force {
			h.Log().Warnf("--force given, using k0s install with --force")
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
				h.Log().Infof("updating service environment")
				return svc.SetEnvironment(ctx, h.Environment)
			})
			if err != nil {
				return err
			}
		}

		if p.IsWet() {
			h.Log().Infof("starting service")
			if err := svc.Start(ctx); err != nil {
				return err
			}
		}

		if NoWait {
			h.Log().Debugf("not waiting because --no-wait given")
			h.Metadata.Ready = true
		} else {
			h.Log().Infof("waiting for node to become ready")

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
