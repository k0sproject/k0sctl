package phase

import (
	"context"
	"fmt"

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
