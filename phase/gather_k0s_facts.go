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
	leader *cluster.Host
}

// Title for the phase
func (p *GatherK0sFacts) Title() string {
	return "Gather k0s facts"
}

// Run the phase
func (p *GatherK0sFacts) Run() error {
	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	if err := controllers.ParallelEach(p.investigateK0s); err != nil {
		return err
	}
	p.leader = p.Config.Spec.K0sLeader()

	if id, err := p.Config.Spec.K0s.GetClusterID(p.leader); err == nil {
		p.Config.Spec.K0s.Metadata.ClusterID = id
		p.SetProp("clusterID", id)
	}

	var workers cluster.Hosts = p.Config.Spec.Hosts.Workers()
	return workers.ParallelEach(p.investigateK0s)
}

func (p *GatherK0sFacts) investigateK0s(h *cluster.Host) error {
	output, err := h.ExecOutput(h.Configurer.K0sCmdf("version"))
	if err != nil {
		return nil
	}

	h.Metadata.K0sBinaryVersion = strings.TrimPrefix(output, "v")
	log.Debugf("%s: has k0s binary version %s", h, h.Metadata.K0sBinaryVersion)

	if h.IsController() && len(p.Config.Spec.K0s.Config) == 0 && h.Configurer.FileExist(h, h.K0sConfigPath()) {
		cfg, err := h.Configurer.ReadFile(h, h.K0sConfigPath())
		if cfg != "" && err == nil {
			log.Infof("%s: found existing configuration", h)
			if err := yaml.Unmarshal([]byte(cfg), &p.Config.Spec.K0s.Config); err != nil {
				return fmt.Errorf("failed to parse existing configuration: %s", err.Error())
			}
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
		log.Debugf("%s: k0s is not running", h)
		return nil
	}

	switch status.Role {
	case "server":
		status.Role = "controller"
	case "server+worker":
		status.Role = "controller+worker"
	}

	if status.Role != h.Role {
		return fmt.Errorf("%s: is configured as k0s %s but is already running as %s - role change is not supported", h, h.Role, status.Role)
	}

	h.Metadata.K0sRunningVersion = strings.TrimPrefix(status.Version, "v")
	if p.Config.Spec.K0s.Version != h.Metadata.K0sRunningVersion {
		return fmt.Errorf("%s: is running k0s %s version %s but target is %s - upgrade is not yet supported", h, h.Role, h.Metadata.K0sRunningVersion, p.Config.Spec.K0s.Version)
	}

	log.Infof("%s: is running k0s %s version %s", h, h.Role, h.Metadata.K0sRunningVersion)

	if !h.IsController() {
		log.Infof("%s: checking if worker %s has joined", p.leader, h.Metadata.Hostname)
		ready, err := p.leader.KubeNodeReady(h)
		if err != nil {
			log.Debugf("%s: failed to get ready status: %s", h, err.Error())
		}
		h.Metadata.Ready = ready
	}

	return nil
}
