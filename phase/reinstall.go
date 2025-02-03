package phase

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

type Reinstall struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *Reinstall) Title() string {
	return "Reinstall"
}

// Prepare the phase
func (p *Reinstall) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return !h.Metadata.K0sInstalled && h.Metadata.K0sRunningVersion != nil && !h.Reset && h.FlagsChanged()
	})

	return nil
}

// ShouldRun is true when there are hosts that needs to be reinstalled
func (p *Reinstall) ShouldRun() bool {
	return cluster.K0sForceFlagSince.Check(p.Config.Spec.K0s.Version) && len(p.hosts) > 0
}

// Run the phase
func (p *Reinstall) Run(_ context.Context) error {
	if !cluster.K0sForceFlagSince.Check(p.Config.Spec.K0s.Version) {
		log.Warnf("k0s version %s does not support install --force flag, installFlags won't be reconfigured", p.Config.Spec.K0s.Version)
		return nil
	}
	controllers := p.hosts.Controllers()
	if len(controllers) > 0 {
		log.Infof("Reinstalling %d controllers sequentially", len(controllers))
		err := controllers.Each(func(h *cluster.Host) error {
			return p.reinstall(h)
		})
		if err != nil {
			return err
		}
	}

	workers := p.hosts.Workers()
	if len(workers) == 0 {
		return nil
	}

	concurrentReinstalls := int(math.Floor(float64(len(p.hosts)) * 0.10))
	if concurrentReinstalls == 0 {
		concurrentReinstalls = 1
	}

	log.Infof("Reinstalling max %d workers in parallel", concurrentReinstalls)

	return p.hosts.BatchedParallelEach(concurrentReinstalls, p.reinstall)
}

func (p *Reinstall) reinstall(h *cluster.Host) error {
	if p.Config.Spec.K0s.DynamicConfig && h.Role != "worker" {
		h.InstallFlags.AddOrReplace("--enable-dynamic-config")
	}

	h.InstallFlags.AddOrReplace("--force=true")

	cmd, err := h.K0sInstallCommand()
	if err != nil {
		return err
	}
	log.Infof("%s: reinstalling k0s", h)
	err = p.Wet(h, fmt.Sprintf("reinstall k0s using `%s", strings.ReplaceAll(cmd, h.Configurer.K0sBinaryPath(), "k0s")), func() error {
		if err := h.Exec(cmd, exec.Sudo(h)); err != nil {
			return fmt.Errorf("failed to reinstall k0s: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = p.Wet(h, "restart k0s service", func() error {
		if err := h.Configurer.RestartService(h, h.K0sServiceName()); err != nil {
			return fmt.Errorf("failed to restart k0s: %w", err)
		}
		log.Infof("%s: waiting for the k0s service to start", h)
		if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.ServiceRunningFunc(h, h.K0sServiceName())); err != nil {
			return fmt.Errorf("k0s did not restart: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("restart after reinstall: %w", err)
	}

	return nil
}
