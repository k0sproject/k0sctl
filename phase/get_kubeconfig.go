package phase

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeconfig is a phase to get and dump the admin kubeconfig
type GetKubeconfig struct {
	GenericPhase
}

// Title for the phase
func (p *GetKubeconfig) Title() string {
	return "Get admin kubeconfig"
}

// Run the phase
func (p *GetKubeconfig) Run() error {
	h := p.Config.Spec.Hosts.Controllers()[0]
	output, err := h.Configurer.ReadFile(h, h.Configurer.KubeconfigPath())
	if err != nil {
		return err
	}
	// the controller admin.conf is aways pointing to localhost, thus we need to change the address
	// something usable from outside
	a := p.Config.Spec.K0s.Config.DigString("spec", "api", "externalAddress")
	if a == "" {
		a = h.SSH.Address
	}

	cfgString, err := kubeConfig(output, p.Config.Metadata.Name, a)
	if err != nil {
		return err
	}
	fmt.Println(cfgString)
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
	cfg.Clusters[name].Server = fmt.Sprintf("https://%s:6443", address)

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
