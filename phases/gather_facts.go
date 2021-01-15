package phases

import (
	"fmt"

	"github.com/k0sproject/k0sctl/phase"

	api "github.com/k0sproject/k0sctl/config"

	log "github.com/sirupsen/logrus"
)

// GatherFacts phase implementation to collect facts (OS, version etc.) from hosts
type GatherFacts struct {
	phase.Analytics
	phase.BasicPhase
}

// Title for the phase
func (p *GatherFacts) Title() string {
	return "Gather Facts"
}

// Run collect all the facts from hosts in parallel
func (p *GatherFacts) Run() error {
	return RunParallelOnHosts(p.Config.Spec.Hosts, p.Config, p.investigateHost)
}

func (p *GatherFacts) investigateHost(h *api.Host, c *api.ClusterConfig) error {
	log.Infof("%s: gathering host facts", h)

	h.Metadata = &api.HostMetadata{}

	fmt.Println("h.Configurer ---->", h.Configurer)
	if err := h.Configurer.CheckPrivilege(); err != nil {
		return err
	}

	arch, err := h.Configurer.Arch()
	if err != nil {
		return err
	}

	h.Metadata.Arch = arch

	isys, err := h.Configurer.InitSystem()
	if err != nil {
		return err
	}

	h.InitSystem = isys

	version, err := h.Configurer.K0sExecutableVersion()
	if err != nil {
		log.Infof("%s: K0s is not installed", h)
	} else {
		log.Infof("%s: is running k0s version %s", h, version)
		h.Metadata.K0sVersion = version
	}

	log.Infof("%s: is running \"%s\"", h, h.Metadata.Os.Name)

	log.Infof("%s: gathered all facts", h)

	return nil
}

// RunParallelOnHosts runs a function parallelly on the listed hosts
func RunParallelOnHosts(hosts api.Hosts, config *api.ClusterConfig, action func(h *api.Host, config *api.ClusterConfig) error) error {
	return hosts.ParallelEach(func(h *api.Host) error {
		err := action(h, config)
		if err != nil {
			log.Error(err.Error())
		}
		return err
	})
}
