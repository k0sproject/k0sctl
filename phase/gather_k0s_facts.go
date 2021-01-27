package phase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type k0sstatus struct {
	Pid      int    `json:"Pid"`
	PPid     int    `json:"PPid"`
	Version  string `json:"Version"`
	Role     string `json:"Role"`
	SysInit  string `json:"SysInit"`
	StubFile string `json:"StubFile"`
}

// GatherK0sFacts gathers information about hosts, such as if k0s is already up and running
type GatherK0sFacts struct {
	GenericPhase
}

// Title for the phase
func (p *GatherK0sFacts) Title() string {
	return "Gather k0s facts"
}

// Run the phase
func (p *GatherK0sFacts) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(p.investigateK0s)
}

func (p *GatherK0sFacts) investigateK0s(h *cluster.Host) error {
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

	if h.Role == "server" && len(p.Config.Spec.K0s.Config) == 0 && h.Configurer.FileExist(h.K0sConfigPath()) {
		cfg, err := h.Configurer.ReadFile(h.K0sConfigPath())
		if cfg != "" && err == nil {
			log.Infof("%s: found existing configuration", h)
			if err := yaml.Unmarshal([]byte(cfg), &p.Config.Spec.K0s.Config); err != nil {
				return fmt.Errorf("failed to parse existing configuration: %s", err.Error())
			}
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

	if status.Version == "" || status.Role == "" || status.Pid == 0 {
		log.Infof("%s: k0s is not running", h)
		return nil
	}

	h.Metadata.K0sRunningVersion = strings.TrimPrefix(status.Version, "v")
	log.Infof("%s: is running a k0s %s version %s", h, h.Role, h.Metadata.K0sRunningVersion)

	return nil
}
