package phase

import (
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// Connect connects to each of the hosts
type Connect struct {
	GenericPhase
}

func (p *Connect) Title() string {
	return "Connect to hosts"
}

func (p *Connect) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: connecting", h)
		if err := h.Connect(); err != nil {
			log.Errorf("%s: failed to connect: %s", h, err.Error())
			return err
		}
		log.Infof("%s: connected", h)

		return h.Connect()
	})
}
