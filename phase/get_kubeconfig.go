package phase

import (
	"context"
	"fmt"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	"k8s.io/client-go/tools/clientcmd"

	log "github.com/sirupsen/logrus"
)

// GetKubeconfig is a phase to get and dump the admin kubeconfig
type GetKubeconfig struct {
	GenericPhase
	APIAddress string
	User       string
	Cluster    string
}

// Title for the phase
func (p *GetKubeconfig) Title() string {
	return "Get admin kubeconfig"
}

var readKubeconfig = func(h *cluster.Host) (string, error) {
	dataDir := h.Configurer.HostPath(h.K0sDataDir())
	log.Debugf("%s: running %v", h, h.Configurer.K0sCmdf("kubeconfig admin --data-dir=%s", h.Configurer.Quote(dataDir)))
	output, err := h.ExecOutput(h.Configurer.K0sCmdf("kubeconfig admin --data-dir=%s", h.Configurer.Quote(dataDir)), exec.Sudo(h), exec.HideOutput())
	if err != nil {
		return "", fmt.Errorf("get kubeconfig from host: %w", err)
	}
	return output, nil
}

func (p *GetKubeconfig) DryRun() error {
	p.DryMsg(p.Config.Spec.Hosts.Controllers()[0], "get admin kubeconfig")
	return nil
}

// Run the phase
func (p *GetKubeconfig) Run(_ context.Context) error {
	h := p.Config.Spec.Hosts.Controllers()[0]

	output, err := readKubeconfig(h)
	if err != nil {
		return fmt.Errorf("read kubeconfig from host: %w", err)
	}

	if p.APIAddress == "" {
		p.APIAddress = p.Config.Spec.KubeAPIURL()
	}

	if p.User != "" {
		p.Config.Metadata.User = p.User
	}

	if p.Cluster != "" {
		p.Config.Metadata.Name = p.Cluster
	}

	cfgString, err := kubeConfig(output, p.Config.Metadata.Name, p.APIAddress, p.Config.Metadata.User)
	if err != nil {
		return err
	}

	p.Config.Metadata.Kubeconfig = cfgString

	return nil
}

// kubeConfig reads in the raw kubeconfig and changes the given address
// and cluster name into it
func kubeConfig(raw string, name string, address, user string) (string, error) {
	cfg, err := clientcmd.Load([]byte(raw))
	if err != nil {
		return "", err
	}

	cfg.Clusters[name] = cfg.Clusters["local"]
	delete(cfg.Clusters, "local")
	cfg.Clusters[name].Server = address

	cfg.Contexts[name] = cfg.Contexts["Default"]
	delete(cfg.Contexts, "Default")
	cfg.Contexts[name].Cluster = name
	cfg.Contexts[name].AuthInfo = user

	cfg.CurrentContext = name

	cfg.AuthInfos[user] = cfg.AuthInfos["user"]
	delete(cfg.AuthInfos, "user")

	out, err := clientcmd.Write(*cfg)
	if err != nil {
		return "", err
	}

	return string(out), nil
}
