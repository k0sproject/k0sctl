package phase

import (
	"github.com/k0sproject/k0sctl/config/cluster"
	_ "github.com/k0sproject/k0sctl/configurer"
	_ "github.com/k0sproject/k0sctl/configurer/linux"
	_ "github.com/k0sproject/k0sctl/configurer/linux/enterpriselinux"

	log "github.com/sirupsen/logrus"
)

// Connect connects to each of the hosts
type DetectOS struct {
	GenericPhase
}

func (p *DetectOS) Title() string {
	return "Detect host operating systems"
}

func (p *DetectOS) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: detecting operating system", h)
		if err := h.ResolveConfigurer(); err != nil {
			return err
		}
		log.Infof("%s: is running %s", h, h.OSVersion.String())
		err := h.Configurer.CheckPrivilege()
		if err != nil {
			return err
		}

		return nil
	})
}
