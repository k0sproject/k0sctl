package phase

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// ConfigureK0s writes the k0s configuration to host k0s config dir
type ConfigureK0s struct {
	GenericPhase
	leader *cluster.Host
}

// Title returns the phase title
func (p *ConfigureK0s) Title() string {
	return "Configure k0s"
}

// Prepare the phase
func (p *ConfigureK0s) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()
	return nil
}

// Run the phase
func (p *ConfigureK0s) Run() error {
	if len(p.Config.Spec.K0s.Config) == 0 {
		p.SetProp("default-config", true)
		log.Warnf("%s: generating default configuration", p.leader)

		var cmd string
		if p.leader.Exec(p.leader.Configurer.K0sCmdf("config create --help"), exec.Sudo(p.leader)) == nil {
			cmd = p.leader.Configurer.K0sCmdf("config create")
		} else {
			cmd = p.leader.Configurer.K0sCmdf("default-config")
		}

		cfg, err := p.leader.ExecOutput(cmd, exec.Sudo(p.leader))
		if err != nil {
			return err
		}

		if err := yaml.Unmarshal([]byte(cfg), &p.Config.Spec.K0s.Config); err != nil {
			return err
		}
	} else {
		p.SetProp("default-config", false)
	}

	controllers := p.Config.Spec.Hosts.Controllers()
	return controllers.BatchedParallelEach(concurrentWorkers, p.configureK0s)
}

func (p *ConfigureK0s) validateConfig(h *cluster.Host) error {
	log.Infof("%s: validating configuration", h)
	var cmd string
	if h.Exec(h.Configurer.K0sCmdf("config validate --help"), exec.Sudo(h)) == nil {
		cmd = h.Configurer.K0sCmdf(`config validate --config "%s"`, h.K0sConfigPath())
	} else {
		cmd = h.Configurer.K0sCmdf(`validate config --config "%s"`, h.K0sConfigPath())
	}

	output, err := h.ExecOutput(cmd, exec.Sudo(h))
	if err != nil {
		return fmt.Errorf("spec.k0s.config fails validation:\n%s", output)
	}

	return nil
}

func (p *ConfigureK0s) configureK0s(h *cluster.Host) error {
	path := h.K0sConfigPath()
	var oldcfg string
	if h.Configurer.FileExist(h, path) {
		c, err := h.Configurer.ReadFile(h, path)
		if err != nil {
			return err
		}
		oldcfg = c

		if !h.Configurer.FileContains(h, path, " generated-by-k0sctl") {
			newpath := path + ".old"
			log.Warnf("%s: an existing config was found and will be backed up as %s", h, newpath)
			if err := h.Configurer.MoveFile(h, path, newpath); err != nil {
				return err
			}
		}
	}

	log.Debugf("%s: writing k0s configuration", h)
	cfg, err := p.configFor(h)
	if err != nil {
		return err
	}

	if err := h.Configurer.WriteFile(h, h.K0sConfigPath(), cfg, "0600"); err != nil {
		return err
	}

	if err := p.validateConfig(h); err != nil {
		return err
	}

	if equalConfig(oldcfg, cfg) {
		log.Debugf("%s: configuration did not change", h)
	} else {
		log.Infof("%s: configuration was changed", h)
		if h.Metadata.K0sRunningVersion != "" && !h.Metadata.NeedsUpgrade {
			log.Infof("%s: restarting the k0s service", h)
			if err := h.Configurer.RestartService(h, h.K0sServiceName()); err != nil {
				return err
			}

			log.Infof("%s: waiting for the k0s service to start", h)
			return h.WaitK0sServiceRunning()
		}
	}

	return nil
}

func equalConfig(a, b string) bool {
	return removeComment(a) == removeComment(b)
}

func removeComment(in string) string {
	var out bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(in))
	for scanner.Scan() {
		row := scanner.Text()
		if !strings.HasPrefix(row, "#") {
			fmt.Fprintln(&out, row)
		}
	}
	return out.String()
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
	var cfg dig.Mapping
	// Leader will get a full config on initialize only
	if !p.Config.Spec.K0s.DynamicConfig || (h == p.leader && h.Metadata.K0sRunningVersion == "") {
		cfg = p.Config.Spec.K0s.Config.Dup()
	} else {
		cfg = p.Config.Spec.K0s.NodeConfig()
	}

	var sans []string

	var addr string
	if h.PrivateAddress != "" {
		addr = h.PrivateAddress
	} else {
		addr = h.Address()
	}
	cfg.DigMapping("spec", "api")["address"] = addr
	addUnlessExist(&sans, addr)

	oldsans := cfg.Dig("spec", "api", "sans")
	switch oldsans := oldsans.(type) {
	case []interface{}:
		for _, v := range oldsans {
			if s, ok := v.(string); ok {
				addUnlessExist(&sans, s)
			}
		}
	case []string:
		for _, v := range oldsans {
			addUnlessExist(&sans, v)
		}
	}

	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	for _, c := range controllers {
		addUnlessExist(&sans, c.Address())
		if c.PrivateAddress != "" {
			addUnlessExist(&sans, c.PrivateAddress)
		}
	}
	addUnlessExist(&sans, "127.0.0.1")
	cfg.DigMapping("spec", "api")["sans"] = sans

	if cfg.Dig("spec", "storage", "etcd", "peerAddress") != nil || h.PrivateAddress != "" {
		cfg.DigMapping("spec", "storage", "etcd")["peerAddress"] = addr
	}

	if _, ok := cfg["apiVersion"]; !ok {
		cfg["apiVersion"] = "k0s.k0sproject.io/v1beta1"
	}

	if _, ok := cfg["kind"]; !ok {
		cfg["kind"] = "ClusterConfig"
	}

	c, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("# generated-by-k0sctl %s\n%s", time.Now().Format(time.RFC3339), c), nil
}
