package phase

import (
	"fmt"
	"time"

	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// ConfigureK0s writes the k0s configuration to host k0s config dir
type ConfigureK0s struct {
	GenericPhase
	k0sconfig string
}

// Title returns the phase title
func (p *ConfigureK0s) Title() string {
	return "Configure K0s"
}

// Run the phase
func (p *ConfigureK0s) Run() error {
	if len(p.Config.Spec.K0s.Config) == 0 {
		p.SetProp("default-config", true)
		leader := p.Config.Spec.K0sLeader()
		log.Warnf("%s: generating default configuration", leader)
		cfg, err := leader.ExecOutput(leader.Configurer.K0sCmdf("default-config"))
		if err != nil {
			return err
		}

		if err := yaml.Unmarshal([]byte(cfg), &p.Config.Spec.K0s.Config); err != nil {
			return err
		}
	} else {
		p.SetProp("default-config", false)
	}

	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	if err := controllers.ParallelEach(p.configureK0s); err != nil {
		return err
	}

	return p.validateConfig(p.Config.Spec.K0sLeader())
}

func (p *ConfigureK0s) validateConfig(h *cluster.Host) error {
	log.Infof("%s: validating configuration", h)
	output, err := h.ExecOutput(h.Configurer.K0sCmdf(`validate config -c "%s"`, h.K0sConfigPath()))
	if err != nil {
		return fmt.Errorf("spec.k0s.config fails validation:\n%s", output)
	}

	return nil
}

func (p *ConfigureK0s) configureK0s(h *cluster.Host) error {
	path := h.K0sConfigPath()
	if h.Configurer.FileExist(h, path) && !h.Configurer.FileContains(h, path, " generated-by-k0sctl") {
		newpath := path + ".old"
		log.Warnf("%s: an existing config was found and will be backed up as %s", h, newpath)
		if err := h.Configurer.MoveFile(h, path, newpath); err != nil {
			return err
		}
	}

	log.Infof("%s: writing k0s config", h)
	cfg, err := p.configFor(h)
	if err != nil {
		return err
	}
	return h.Configurer.WriteFile(h, h.K0sConfigPath(), cfg, "0700")
}

func (p *ConfigureK0s) configFor(h *cluster.Host) (string, error) {
	var addr string
	if h.PrivateAddress != "" {
		addr = h.PrivateAddress
	} else {
		addr = h.Address()
	}

	cfg := p.Config.Spec.K0s.Config

	// ...ok then....
	spec, ok := cfg["spec"].(cluster.Mapping)
	if !ok {
		spec = cluster.Mapping{}
		cfg["spec"] = &spec
	}

	api, ok := spec["api"].(cluster.Mapping)
	if !ok {
		api = cluster.Mapping{}
		spec["api"] = &api
	}

	api["address"] = addr
	sans, ok := api["sans"].(*[]string)
	if !ok {
		sans = &[]string{}
		api["sans"] = sans
	}

	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	for _, c := range controllers {
		var caddr string
		if c.PrivateAddress != "" {
			caddr = c.PrivateAddress
		} else {
			caddr = c.Address()
		}

		var found bool
		for _, s := range *sans {
			if s == caddr {
				found = true
				break
			}
		}
		if !found {
			*sans = append(*sans, caddr)
		}
	}
	*sans = append(*sans, "127.0.0.1")

	if storage, ok := spec["storage"].(cluster.Mapping); ok {
		if etcd, ok := storage["etcd"].(cluster.Mapping); ok {
			etcd["peerAddress"] = addr
		}
	}

	c, err := yaml.Marshal(p.Config.Spec.K0s.Config)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("# generated-by-k0sctl %s\n%s", time.Now().Format(time.RFC3339), c), nil
}
