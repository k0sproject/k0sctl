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
	SkipDowngradeCheck bool
}

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

func (p *ValidateFacts) validateVersionSkew() error {
	return p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.Metadata.NeedsUpgrade
	}).Each(context.Background(), func(_ context.Context, h *cluster.Host) error {
		log.Debugf("%s: validating k0s version skew", h)
		delta := version.NewDelta(h.Metadata.K0sRunningVersion, p.Config.Spec.K0s.Version)
		log.Debugf("%s: version delta: %s", h, delta)

		var unacceptable bool
		switch {
		case delta.MinorUpgrade:
			if h.IsController() {
				if p.Config.Spec.K0s.Version.Segments()[1]-h.Metadata.K0sRunningVersion.Segments()[1] > 1 {
					log.Debugf("%s: controller upgrade not within version skew policy", h)
					unacceptable = true
				}
			} else if p.Config.Spec.K0s.Version.Segments()[1]-h.Metadata.K0sRunningVersion.Segments()[1] > 3 {
				log.Debugf("%s: worker upgrade not within version skew policy", h)
				unacceptable = true
			}

			if !unacceptable {
				log.Debugf("%s: minor upgrade within acceptable skew", h)
			}
		case delta.MajorUpgrade:
			unacceptable = true
			log.Warnf("%s: major upgrades are not supported, the operation will highly likely fail", h)
		}

		if unacceptable {
			if Force {
				log.Warnf("upgrade from %s directly to %s is not within the version skew policy, allowing because --force given", h.Metadata.K0sRunningVersion, p.Config.Spec.K0s.Version)
				return nil
			}
			return fmt.Errorf("upgrade from %s directly to %s is not within the version skew policy, you can use --force to skip this check", h.Metadata.K0sRunningVersion, p.Config.Spec.K0s.Version)
		}

		log.Debugf("%s: version skew check passed", h)
		return nil
	})
}
