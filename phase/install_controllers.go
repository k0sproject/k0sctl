package phase

import (
	"bytes"
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

// InstallControllers installs k0s controllers and joins them to the cluster
type InstallControllers struct {
	GenericPhase
	hosts      cluster.Hosts
	leader     *cluster.Host
	numRunning int
}

// Title for the phase
func (p *InstallControllers) Title() string {
	return "Install controllers"
}

// Prepare the phase
func (p *InstallControllers) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()
	var countRunning int
	p.hosts = p.Config.Spec.Hosts.Controllers().Filter(func(h *cluster.Host) bool {
		if h.Metadata.K0sRunningVersion != nil {
			countRunning++
		}
		return !h.Reset && !h.Metadata.NeedsUpgrade && (h != p.leader && h.Metadata.K0sRunningVersion == nil)
	})
	p.numRunning = countRunning
	return nil
}

// ShouldRun is true when there are controllers
func (p *InstallControllers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Before runs "before install" hooks for controller hosts
func (p *InstallControllers) Before() error {
	if len(p.hosts) == 0 {
		return nil
	}
	return p.runHooks(context.Background(), "install", "before", p.hosts...)
}

// CleanUp cleans up the environment override files on hosts
func (p *InstallControllers) CleanUp() {
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
			if err := h.Exec(h.K0sResetCommand(), exec.Sudo(h)); err != nil {
				log.Warnf("%s: k0s reset failed", h)
			}
		}
		return nil
	})
}

// After runs "after install" hooks for controller hosts and cleans up tokens
func (p *InstallControllers) After() error {
	// Run "after install" hooks for controllers first
	if err := p.runHooks(context.Background(), "install", "after", p.hosts...); err != nil {
		return err
	}
	for i, h := range p.hosts {
		if h.Metadata.K0sTokenData.Token == "" {
			continue
		}
		h.Metadata.K0sTokenData.Token = ""
		err := p.Wet(p.leader, fmt.Sprintf("invalidate k0s join token for controller %s", h), func() error {
			log.Debugf("%s: invalidating join token for controller %d", p.leader, i+1)
			return p.leader.Exec(p.leader.Configurer.K0sCmdf("token invalidate --data-dir=%s %s", p.leader.K0sDataDir(), h.Metadata.K0sTokenData.ID), exec.Sudo(p.leader))
		})
		if err != nil {
			log.Warnf("%s: failed to invalidate controller join token: %v", p.leader, err)
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
func (p *InstallControllers) Run(ctx context.Context) error {
	for _, h := range p.hosts {
		if p.IsWet() {
			log.Infof("%s: generate join token for %s", p.leader, h)
			token, err := p.Config.Spec.K0s.GenerateToken(
				ctx,
				p.leader,
				"controller",
				30*time.Minute,
			)
			if err != nil {
				return err
			}
			tokenData, err := cluster.ParseToken(token)
			if err != nil {
				return err
			}
			h.Metadata.K0sTokenData = tokenData
		} else {
			p.DryMsgf(p.leader, "generate a k0s join token for controller %s", h)
			h.Metadata.K0sTokenData.ID = "dry-run"
			h.Metadata.K0sTokenData.URL = p.Config.Spec.KubeAPIURL()
		}
	}
	err := p.parallelDo(ctx, p.hosts, func(_ context.Context, h *cluster.Host) error {
		if p.IsWet() || !p.leader.Metadata.DryRunFakeLeader {
			log.Infof("%s: validating api connection to %s", h, h.Metadata.K0sTokenData.URL)
			if err := retry.WithDefaultTimeout(ctx, node.HTTPStatusFunc(h, h.Metadata.K0sTokenData.URL, 200, 401, 404)); err != nil {
				return fmt.Errorf("failed to connect from controller to kubernetes api - check networking: %w", err)
			}
		} else {
			log.Warnf("%s: dry-run: skipping api connection validation to because cluster is not actually running", h)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// just one controller to install, install it and return
	if len(p.hosts) == 1 {
		log.Debug("only one controller to install")
		return p.installK0s(ctx, p.hosts[0])
	}

	if p.manager.Concurrency < 2 {
		log.Debugf("installing %d controllers sequantially because concurrency is set to 1", len(p.hosts))
		return p.hosts.Each(ctx, p.installK0s)
	}

	var remaining cluster.Hosts
	remaining = append(remaining, p.hosts...)

	if p.numRunning == 1 && len(remaining) >= 2 {
		perBatch := min(2, p.manager.Concurrency)
		firstBatch := remaining[:perBatch]

		log.Debugf("installing first %d controllers to reach HA state and quorum", perBatch)
		if err := firstBatch.BatchedParallelEach(ctx, perBatch, p.installK0s); err != nil {
			return err
		}
		remaining = remaining[perBatch:]
		p.numRunning += perBatch

		if len(remaining) == 0 {
			log.Debug("all controllers installed")
			return nil
		}
		log.Debugf("remaining %d controllers to install", len(remaining))
	}

	if p.numRunning%2 == 0 {
		log.Debug("even number of running controllers, installing one first to reach quorum")
		if err := p.installK0s(ctx, remaining[0]); err != nil {
			return err
		}
		remaining = remaining[1:]
		p.numRunning++
	}

	// install the rest in parallel in uneven quorum-optimized batches
	for len(remaining) > 0 {
		currentTotal := p.numRunning + len(remaining)
		quorum := (currentTotal / 2) + 1
		safeMax := (quorum / 2)
		if safeMax < 1 {
			safeMax = 1
		}

		perBatch := min(safeMax, p.manager.Concurrency, len(remaining))

		log.Debugf("installing next %d controllers (quorum=%d, total=%d)", perBatch, quorum, currentTotal)

		batch := remaining[:perBatch]
		if err := batch.BatchedParallelEach(ctx, perBatch, p.installK0s); err != nil {
			return err
		}

		remaining = remaining[perBatch:]
		p.numRunning += perBatch
	}
	log.Debug("all controllers installed")
	return nil
}

func (p *InstallControllers) installK0s(ctx context.Context, h *cluster.Host) error {
	tokenPath := h.K0sJoinTokenPath()
	log.Infof("%s: writing join token to %s", h, tokenPath)
	err := p.Wet(h, fmt.Sprintf("write k0s join token to %s", tokenPath), func() error {
		return h.Configurer.WriteFile(h, tokenPath, h.Metadata.K0sTokenData.Token, "0600")
	})
	if err != nil {
		return err
	}

	if p.Config.Spec.K0s.DynamicConfig {
		h.InstallFlags.AddOrReplace("--enable-dynamic-config")
	}

	if Force {
		log.Warnf("%s: --force given, using k0s install with --force", h)
		h.InstallFlags.AddOrReplace("--force=true")
	}

	cmd, err := h.K0sInstallCommand()
	if err != nil {
		return err
	}
	log.Infof("%s: installing k0s controller", h)

	err = p.Wet(h, fmt.Sprintf("install k0s controller using `%s", strings.ReplaceAll(cmd, h.K0sInstallLocation(), "k0s")), func() error {
		var stdout, stderr bytes.Buffer
		runner, err := h.ExecStreams(cmd, nil, &stdout, &stderr, exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("run k0s install: %w", err)
		}
		if err := runner.Wait(); err != nil {
			log.Errorf("%s: k0s install failed: %s %s", h, stdout.String(), stderr.String())
			return fmt.Errorf("k0s install failed: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}
	h.Metadata.K0sInstalled = true
	h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version

	if p.IsWet() {
		if len(h.Environment) > 0 {
			log.Infof("%s: updating service environment", h)
			if err := h.Configurer.UpdateServiceEnvironment(h, h.K0sServiceName(), h.Environment); err != nil {
				return err
			}
		}

		log.Infof("%s: starting service", h)
		if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
			return err
		}

		log.Infof("%s: waiting for the k0s service to start", h)
		if err := retry.WithDefaultTimeout(ctx, node.ServiceRunningFunc(h, h.K0sServiceName())); err != nil {
			return err
		}

		err := retry.WithDefaultTimeout(ctx, func(_ context.Context) error {
			out, err := h.ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get --raw='/readyz?verbose=true'"), exec.Sudo(h))
			if err != nil {
				return fmt.Errorf("readiness endpoint reports %q: %w", out, err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("controller did not reach ready state: %w", err)
		}

		h.Metadata.Ready = true
	}

	return nil
}
