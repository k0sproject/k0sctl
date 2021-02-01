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

	log.Debugf("%s: writing k0s config", h)
	cfg, err := p.configFor(h)
	if err != nil {
		return err
	}
	return h.Configurer.WriteFile(h, h.K0sConfigPath(), cfg, "0700")
}

func addUnlessExist(slice *[]string, s string) {
	var found bool
	for _, v := range *slice {
		if v == s {
			found = true
			break
		}
	}
	if !found {
		*slice = append(*slice, s)
	}
}

func (p *ConfigureK0s) configFor(h *cluster.Host) (string, error) {
	var addr string
	if h.PrivateAddress != "" {
		addr = h.PrivateAddress
	} else {
		addr = h.Address()
	}

	cfg := p.Config.Spec.K0s.Config

	cfg.DigMapping("spec", "api")["address"] = addr
	var sans []string
	oldsans, ok := cfg.Dig("spec", "api", "sans").([]interface{})
	if ok {
		for _, v := range oldsans {
			if s, ok := v.(string); ok {
				addUnlessExist(&sans, s)
			}
		}
	}

	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	for _, c := range controllers {
		var caddr string
		if c.PrivateAddress != "" {
			caddr = c.PrivateAddress
		} else {
			caddr = c.Address()
		}
		addUnlessExist(&sans, caddr)
	}
	addUnlessExist(&sans, "127.0.0.1")
	cfg.DigMapping("spec", "api")["sans"] = sans

	if cfg.Dig("spec", "storage", "etcd", "peerAddress") != nil {
		cfg.DigMapping("spec", "storage", "etcd")["peerAddress"] = addr
	}

	c, err := yaml.Marshal(p.Config.Spec.K0s.Config)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("# generated-by-k0sctl %s\n%s", time.Now().Format(time.RFC3339), c), nil
}
