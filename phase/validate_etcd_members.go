package phase

import (
	"fmt"
	"slices"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	log "github.com/sirupsen/logrus"
)

// ValidateEtcdMembers checks for existing etcd members with the same IP as a new controller
type ValidateEtcdMembers struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *ValidateEtcdMembers) Title() string {
	return "Validate etcd members"
}

// Prepare the phase
func (p *ValidateEtcdMembers) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Controllers().Filter(func(h *cluster.Host) bool {
		return h.Metadata.K0sRunningVersion == nil // only check new controllers
	})

	return nil
}

// ShouldRun is true when there are new controllers and etcd
func (p *ValidateEtcdMembers) ShouldRun() bool {
	if p.Config.Spec.K0sLeader().Metadata.K0sRunningVersion == nil {
		log.Debugf("%s: leader has no k0s running, assuming a fresh cluster", p.Config.Spec.K0sLeader())
		return false
	}

	if p.Config.Spec.K0sLeader().Role == "single" {
		log.Debugf("%s: leader is a single node, assuming no etcd", p.Config.Spec.K0sLeader())
		return false
	}

	if len(p.Config.Spec.K0s.Config) > 0 {
		storageType := p.Config.Spec.K0s.Config.DigString("spec", "storage", "type")
		if storageType != "" && storageType != "etcd" {
			log.Debugf("%s: storage type is %q, not k0s managed etcd", p.Config.Spec.K0sLeader(), storageType)
			return false
		}
	}
	return len(p.hosts) > 0
}

// Run the phase
func (p *ValidateEtcdMembers) Run() error {
	if err := p.validateControllerSwap(); err != nil {
		return err
	}

	return nil
}

func (p *ValidateEtcdMembers) validateControllerSwap() error {
	if len(p.Config.Metadata.EtcdMembers) > len(p.Config.Spec.Hosts.Controllers()) {
		log.Warnf("there are more etcd members in the cluster than controllers listed in the configuration")
	}

	for _, h := range p.hosts {
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
