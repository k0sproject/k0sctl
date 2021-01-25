package phase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

type k0sstatus struct {
	Pid      int    `json:"Pid"`
	PPid     int    `json:"PPid"`
	Version  string `json:"Version"`
	Role     string `json:"Role"`
	SysInit  string `json:"SysInit"`
	StubFile string `json:"StubFile"`
}

// GatherFacts gathers information about hosts, such as if k0s is already up and running
type GatherFacts struct {
	GenericPhase
}

// Title for the phase
func (p *GatherFacts) Title() string {
	return "Gather host facts"
}

// Run the phase
func (p *GatherFacts) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(p.investigateHost)
}

func (p *GatherFacts) investigateHost(h *cluster.Host) error {
	log.Infof("%s: investigating host", h)
	p.IncProp(h.Role)

	if err := p.investigateK0s(h); err != nil {
		return err
	}

	output, err := h.Configurer.Arch()
	if err != nil {
		return err
	}
	h.Metadata.Arch = output
	p.IncProp(h.Metadata.Arch)
	log.Infof("%s: cpu architecture is %s", h, h.Metadata.Arch)

	return nil
}

func (p *GatherFacts) investigateK0s(h *cluster.Host) error {
	if !h.Configurer.ServiceIsRunning("k0s") {
		log.Infof("%s: k0s service is not running", h)
		return nil
	}

	output, err := h.ExecOutput(h.Configurer.K0sCmdf("version"))
	if err != nil {
		return nil
	}

	h.Metadata.K0sBinaryVersion = strings.TrimPrefix(output, "v")
	log.Infof("%s: has k0s binary version %s", h, h.Metadata.K0sBinaryVersion)

	if h.Role == "server" && h.Configurer.FileExist(h.K0sJoinTokenPath()) {
		token, err := h.Configurer.ReadFile(h.K0sJoinTokenPath())
		if token != "" && err == nil {
			log.Infof("%s: found an existing controller token", h)
			p.Config.Spec.K0s.Metadata.ControllerToken = token
		}
	}

	if h.Role == "worker" && h.Configurer.FileExist(h.K0sJoinTokenPath()) {
		token, err := h.Configurer.ReadFile(h.K0sJoinTokenPath())
		if token != "" && err == nil {
			log.Infof("%s: found an existing worker token", h)
			p.Config.Spec.K0s.Metadata.WorkerToken = token
		}
	}

	output, err = h.ExecOutput(h.Configurer.K0sCmdf("status -o json"))
	if err != nil {
		return nil
	}

	status := k0sstatus{}

	if err := json.Unmarshal([]byte(output), &status); err != nil {
		log.Warnf("%s: failed to decode k0s status output: %s", h, err.Error())
		return nil
	}

	if status.Version == "" || status.Role == "" {
		return fmt.Errorf("host was configured as a k0s %s but is already running as %s", h.Role, status.Role)
	}

	h.Metadata.K0sRunningVersion = strings.TrimPrefix(status.Version, "v")
	log.Infof("%s: is running a k0s %s version %s", h, h.Role, h.Metadata.K0sRunningVersion)

	return nil
}
