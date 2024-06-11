package phase

import (
	"fmt"
	"strings"

	"github.com/alessio/shellescape"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// GetKubeconfig is a phase to get and dump the admin kubeconfig
type GetKubeconfig struct {
	GenericPhase
	APIAddress string
}

type kubeconfig struct {
	Clusters []struct {
		Cluster struct {
			Server string `yaml:"server"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
}

// Title for the phase
func (p *GetKubeconfig) Title() string {
	return "Get admin kubeconfig"
}

func (p *GetKubeconfig) DryRun() error {
	p.DryMsg(p.Config.Spec.Hosts.Controllers()[0], "get admin kubeconfig")
	return nil
}

// Run the phase
func (p *GetKubeconfig) Run() error {
	h := p.Config.Spec.K0sLeader()

	output, err := h.ExecOutput(h.Configurer.K0sCmdf("kubeconfig admin --config=%s", shellescape.Quote(h.Configurer.KubeconfigPath(h, h.K0sDataDir()))), exec.Sudo(h))
	if err != nil {
		return fmt.Errorf("read kubeconfig from host: %w", err)
	}

	if p.APIAddress != "" {
		log.Debugf("%s: replacing api address with %v", h, p.APIAddress)
		kubeconf := kubeconfig{}
		if err := yaml.Unmarshal([]byte(output), &kubeconf); err != nil {
			return fmt.Errorf("unmarshal kubeconfig: %w", err)
		}
		if len(kubeconf.Clusters) == 0 {
			return fmt.Errorf("no clusters found in kubeconfig")
		}
		server := kubeconf.Clusters[0].Cluster.Server
		if server == "" {
			return fmt.Errorf("no server found in kubeconfig")
		}
		log.Debugf("%s: replacing %v with %v", h, server, p.APIAddress)
		output = strings.ReplaceAll(output, server, p.APIAddress)
	}

	p.Config.Metadata.Kubeconfig = output

	return nil
}
