package phase

import (
	"fmt"
	"strings"

	"github.com/alessio/shellescape"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/v2/exec"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeconfig is a phase to get and dump the admin kubeconfig
type GetKubeconfig struct {
	GenericPhase
	APIAddress string
}

// Title for the phase
func (p *GetKubeconfig) Title() string {
	return "Get admin kubeconfig"
}

var readKubeconfig = func(h *cluster.Host) (string, error) {
	output, err := h.ExecOutput(h.Configurer.K0sCmdf("kubeconfig admin --data-dir=%s", shellescape.Quote(h.K0sDataDir())), exec.Sudo(h), exec.HideOutput())
	if err != nil {
		return "", fmt.Errorf("get kubeconfig from host: %w", err)
	}
	return output, nil
}

var k0sConfig = func(h *cluster.Host) (dig.Mapping, error) {
	cfgContent, err := h.Configurer.ReadFile(h, h.Configurer.K0sConfigPath())
	if err != nil {
		return nil, fmt.Errorf("read k0s config from host: %w", err)
	}

	var cfg dig.Mapping
	if err := yaml.Unmarshal([]byte(cfgContent), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal k0s config: %w", err)
	}

	if err != nil {
		return nil, fmt.Errorf("parse k0s config: %w", err)
	}

	return cfg, nil
}

func (p *GetKubeconfig) DryRun() error {
	p.DryMsg(p.Config.Spec.Hosts.Controllers()[0], "get admin kubeconfig")
	return nil
}

// Run the phase
func (p *GetKubeconfig) Run() error {
	h := p.Config.Spec.Hosts.Controllers()[0]

	cfg, err := k0sConfig(h)
	if err != nil {
		return err
	}

	output, err := readKubeconfig(h)
	if err != nil {
		return fmt.Errorf("read kubeconfig from host: %w", err)
	}

	if p.APIAddress == "" {
		// the controller admin.conf is aways pointing to localhost, thus we need to change the address
		// something usable from outside
		address := h.Address()
		if a, ok := cfg.Dig("spec", "api", "externalAddress").(string); ok && a != "" {
			address = a
		}

		port := 6443
		if p, ok := cfg.Dig("spec", "api", "port").(int); ok && p != 0 {
			port = p
		}

		if strings.Contains(address, ":") {
			p.APIAddress = fmt.Sprintf("https://[%s]:%d", address, port)
		} else {
			p.APIAddress = fmt.Sprintf("https://%s:%d", address, port)
		}
	}

	cfgString, err := kubeConfig(output, p.Config.Metadata.Name, p.APIAddress)
	if err != nil {
		return err
	}

	p.Config.Metadata.Kubeconfig = cfgString

	return nil
}

// kubeConfig reads in the raw kubeconfig and changes the given address
// and cluster name into it
func kubeConfig(raw string, name string, address string) (string, error) {
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
	cfg.Contexts[name].AuthInfo = "admin"

	cfg.CurrentContext = name

	cfg.AuthInfos["admin"] = cfg.AuthInfos["user"]
	delete(cfg.AuthInfos, "user")

	out, err := clientcmd.Write(*cfg)
	if err != nil {
		return "", err
	}

	return string(out), nil
}
