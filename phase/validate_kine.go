package phase

import (
	"fmt"
	"net/url"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	log "github.com/sirupsen/logrus"
)

// ValidateKine performs kine checks
type ValidateKine struct {
	GenericPhase

	controllers     cluster.Hosts
	controllerCount int
	storage         string
}

// Title for the phase
func (p *ValidateKine) Title() string {
	return "Validate kine"
}

func (p *ValidateKine) Prepare(config *v1beta1.Cluster) error {
	p.Config = config

	p.storage = p.Config.Spec.K0s.Config.DigString("storage", "type")
	p.controllerCount = len(p.Config.Spec.Hosts.Controllers())
	p.controllers = p.Config.Spec.Hosts.Controllers()

	return nil
}

func (p *ValidateKine) ShouldRun() bool {
	if p.storage != "kine" {
		log.Debugf("cluster does not use kine")
		return false
	}
	return true
}

// Run the phase
func (p *ValidateKine) Run() error {
	log.Infof("cluster is using %s", p.storage)

	dataSource := p.Config.Spec.K0s.Config.DigString("storage", "kine", "datasource")
	// FIXME: how to check for a path?
	if dataSource == "" || dataSource[0:1] == "/" {
		if p.controllerCount != 1 {
			return fmt.Errorf("cluster with kine sqlite storage should only have one controller")
		}

		log.Info("datasource is empty, cluster will use sqlite")
		return nil
	}

	u, err := url.Parse(dataSource)
	if err != nil {
		return fmt.Errorf("unable to parse data source (%s) %w", dataSource, err)
	}

	p.controllers.ParallelEach(func(h *cluster.Host) error {
		err := h.Execf("nc -z %s %d", u.Hostname(), u.Port())
		if err != nil {
			return fmt.Errorf("host '%s' could not connect to data source: %s", h.PrivateAddress, dataSource)
		}
		log.Info("connection to storage is working")
		return nil
	})

	return nil
}
