package phase

import (
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// Disconnect connects to each of the hosts
type Disconnect struct {
	GenericPhase
}

func (p *Disconnect) Title() string {
	return "Disconnect from hosts"
}

func (p *Disconnect) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: disconnecting", h)
		h.Disconnect()
		return nil
	})
}
