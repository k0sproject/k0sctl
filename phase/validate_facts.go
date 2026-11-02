package phase

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// ValidateFacts performs remote OS detection
type ValidateFacts struct {
	GenericPhase
	SkipDowngradeCheck   bool
	SkipVersionSkewCheck bool
}

// maxKubeletSkew is the number of minor versions a kubelet may be older than
// the kube-apiserver according to the Kubernetes version skew policy.
const maxKubeletSkew = 3

// Title for the phase
func (p *ValidateFacts) Title() string {
	return "Validate facts"
}

// Run the phase
func (p *ValidateFacts) Run(_ context.Context) error {
	if err := p.validateDowngrade(); err != nil {
		return err
	}

	if err := p.validateDefaultVersion(); err != nil {
		return err
	}

	if err := p.validateVersionSkew(); err != nil {
		return err
	}

	if err := p.validateNodeLocalLoadBalancing(); err != nil {
		return err
	}

	return nil
}

func (p *ValidateFacts) validateDowngrade() error {
	if p.SkipDowngradeCheck {
		return nil
	}

	if p.Config.Spec.K0sLeader().Metadata.K0sRunningVersion == nil || p.Config.Spec.K0s.Version == nil {
		return nil
	}

	if p.Config.Spec.K0sLeader().Metadata.K0sRunningVersion.GreaterThan(p.Config.Spec.K0s.Version) {
		return fmt.Errorf("can't perform a downgrade: %s > %s", p.Config.Spec.K0sLeader().Metadata.K0sRunningVersion, p.Config.Spec.K0s.Version)
	}

	return nil
}

// validateVersionSkew enforces the Kubernetes version skew policy for upgrades:
// the control plane can only be upgraded one minor version at a time, and a
// kubelet may not be more than three minor versions older than the control plane.
// See https://kubernetes.io/releases/version-skew-policy/
func (p *ValidateFacts) validateVersionSkew() error {
	if p.SkipVersionSkewCheck {
		return nil
	}

	target := p.Config.Spec.K0s.Version
	if target == nil {
		return nil
	}

	for _, h := range p.Config.Spec.Hosts {
		running := h.Metadata.K0sRunningVersion
		if h.Reset || running == nil {
			continue
		}

		if h.IsController() {
			next := postApplyVersion(h, target)
			if skew, ok := minorVersionSkew(running, next); ok && skew > 1 {
				return fmt.Errorf("%s: can't upgrade the controller from %s to %s: the kubernetes control plane can only be upgraded one minor version at a time, upgrade to v%d.%d first or use --disable-version-skew-check to skip this check", h, running, next, running.Segments()[0], running.Segments()[1]+1)
			}
			continue
		}

		// Workers are upgraded to the configured version in the same apply, except when k0sctl uses the
		// existing k0s binary, which then decides the version the worker runs after the apply.
		if !h.UseExistingK0s {
			continue
		}
		next := postApplyVersion(h, target)
		if skew, ok := minorVersionSkew(next, target); ok && skew > maxKubeletSkew {
			log.Warnf("%s: the worker will run %s because useExistingK0s is set, which is more than %d minor versions older than %s and not supported by the kubernetes version skew policy", h, next, maxKubeletSkew, target)
		}
	}

	return nil
}

// postApplyVersion returns the k0s version the host runs after the apply. Without useExistingK0s, that is
// target. With useExistingK0s, k0sctl only restarts a host that is marked for upgrade, and then it restarts
// on the installed binary; any other host keeps running its current version.
func postApplyVersion(h *cluster.Host, target *version.Version) *version.Version {
	if !h.UseExistingK0s {
		return target
	}
	if h.Metadata.NeedsUpgrade && h.Metadata.K0sBinaryVersion != nil {
		return h.Metadata.K0sBinaryVersion
	}
	return h.Metadata.K0sRunningVersion
}

// minorVersionSkew returns how many minor versions target is ahead of running.
// It returns false when the major versions differ.
func minorVersionSkew(running, target *version.Version) (int, bool) {
	r, t := running.Segments(), target.Segments()
	if r[0] != t[0] {
		return 0, false
	}
	return t[1] - r[1], true
}

func (p *ValidateFacts) validateDefaultVersion() error {
	// Only check when running with a defaulted version
	if !p.Config.Spec.K0s.Metadata.VersionDefaulted {
		return nil
	}

	// Installing a fresh latest is ok
	if p.Config.Spec.K0sLeader().Metadata.K0sRunningVersion == nil {
		return nil
	}

	// Upgrading should not be performed if the config version was defaulted
	if p.Config.Spec.K0s.Version.GreaterThan(p.Config.Spec.K0sLeader().Metadata.K0sRunningVersion) {
		log.Warnf("spec.k0s.version was automatically defaulted to %s but the cluster is running %s", p.Config.Spec.K0s.Version, p.Config.Spec.K0sLeader().Metadata.K0sRunningVersion)
		log.Warnf("to perform an upgrade, set the k0s version in the configuration explicitly")
		p.Config.Spec.K0s.Version = p.Config.Spec.K0sLeader().Metadata.K0sRunningVersion
		for _, h := range p.Config.Spec.Hosts {
			h.Metadata.NeedsUpgrade = false
		}
	}

	return nil
}

func (p *ValidateFacts) validateNodeLocalLoadBalancing() error {
	if p.Config == nil || p.Config.Spec == nil || p.Config.Spec.K0s == nil {
		return nil
	}

	single := p.Config.Spec.Hosts.Find(func(h *cluster.Host) bool {
		return h.Role == "single"
	})
	if single == nil {
		return nil
	}

	if enabled, ok := p.Config.Spec.K0s.Config.Dig("network", "nodeLocalLoadBalancing", "enabled").(bool); ok && enabled {
		return fmt.Errorf("spec.k0s.config.network.nodeLocalLoadBalancing.enabled cannot be true when %s has role 'single'", single)
	}

	return nil
}
