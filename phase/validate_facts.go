package phase

import (
	"fmt"
	"slices"

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
func (p *ValidateFacts) Run() error {
	if err := p.validateDowngrade(); err != nil {
		return err
	}

	if err := p.validateDefaultVersion(); err != nil {
		return err
	}

	if err := p.validateControllerSwap(); err != nil {
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

func (p *ValidateFacts) validateControllerSwap() error {
	log.Debugf("validating controller list vs etcd member list")
	if p.Config.Spec.K0sLeader().Metadata.K0sRunningVersion == nil {
		log.Debugf("%s: leader has no k0s running, assuming a fresh cluster", p.Config.Spec.K0sLeader())
		return nil
	}

	if p.Config.Spec.K0sLeader().Role == "single" {
		log.Debugf("%s: leader is a single node, assuming no etcd", p.Config.Spec.K0sLeader())
		return nil
	}

	if len(p.Config.Metadata.EtcdMembers) > len(p.Config.Spec.Hosts.Controllers()) {
		log.Warnf("there are more etcd members in the cluster than controllers listed in the k0sctl configuration")
	}

	for _, h := range p.Config.Spec.Hosts.Controllers() {
		if h.Metadata.K0sRunningVersion != nil {
			log.Debugf("%s: host has k0s running, no need to check if it was replaced", h)
			continue
		}

		log.Debugf("%s: host is new, checking if etcd members list already contains %s", h, h.PrivateAddress)
		if slices.Contains(p.Config.Metadata.EtcdMembers, h.PrivateAddress) {
			if Force {
				log.Infof("%s: force used, running 'k0s etcd leave' for the host", h)
				leader := p.Config.Spec.K0sLeader()
				leaveCommand := leader.Configurer.K0sCmdf("etcd leave --peer-address %s", h.PrivateAddress)
				err := p.Wet(h, fmt.Sprintf("remove host from etcd using %v", leaveCommand), func() error {
					return leader.Exec(leaveCommand)
				})
				if err != nil {
					return fmt.Errorf("controller %s is listed as an existing etcd member but k0s is not found installed on it, the host may have been replaced. attempted etcd leave for the address %s but it failed: %w", h, h.PrivateAddress, err)
				}
				continue
			}
			return fmt.Errorf("controller %s is listed as an existing etcd member but k0s is not found installed on it, the host may have been replaced. check the host and use `k0s etcd leave --peer-address %s on a controller or re-run apply with --force", h, h.PrivateAddress)
		}
		log.Debugf("%s: no match, assuming its safe to install", h)
	}

	return nil
}
