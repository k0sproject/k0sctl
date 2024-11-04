package phase

import (
	"bytes"
	"context"
	"fmt"
	gopath "path"
	"slices"
	"time"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/version"
	"github.com/sergi/go-diff/diffmatchpatch"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// "k0s default-config" was replaced with "k0s config create" in v1.23.1+k0s.0
var configCreateSinceVersion = version.MustConstraint(">= v1.23.1+k0s.0")

const (
	configSourceExisting int = iota
	configSourceDefault
	configSourceProvided
	configSourceNodeConfig
)

// ConfigureK0s writes the k0s configuration to host k0s config dir
type ConfigureK0s struct {
	GenericPhase
	leader        *cluster.Host
	configSource  int
	newBaseConfig dig.Mapping
	hosts         cluster.Hosts
}

// Title returns the phase title
func (p *ConfigureK0s) Title() string {
	return "Configure k0s"
}

// Prepare the phase
func (p *ConfigureK0s) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()

	if len(p.Config.Spec.K0s.Config) > 0 {
		log.Debug("using provided k0s config")
		p.configSource = configSourceProvided
		p.newBaseConfig = p.Config.Spec.K0s.Config.Dup()
	} else if p.leader.Metadata.K0sExistingConfig != "" {
		log.Debug("using existing k0s config")
		p.configSource = configSourceExisting
		p.newBaseConfig = make(dig.Mapping)
		err := yaml.Unmarshal([]byte(p.leader.Metadata.K0sExistingConfig), &p.newBaseConfig)
		if err != nil {
			return fmt.Errorf("failed to unmarshal existing k0s config: %w", err)
		}
	} else {
		log.Debug("using generated default k0s config")
		p.configSource = configSourceDefault
		cfg, err := p.generateDefaultConfig()
		if err != nil {
			return fmt.Errorf("failed to generate default k0s config: %w", err)
		}
		p.newBaseConfig = make(dig.Mapping)
		err = yaml.Unmarshal([]byte(cfg), &p.newBaseConfig)
		if err != nil {
			return fmt.Errorf("failed to unmarshal default k0s config: %w", err)
		}
	}

	for _, h := range p.Config.Spec.Hosts.Controllers() {
		if h.Reset {
			continue
		}

		cfgNew, err := p.configFor(h)
		if err != nil {
			return fmt.Errorf("failed to build k0s config for %s: %w", h, err)
		}
		tempConfigPath, err := h.Configurer.TempFile(h)
		if err != nil {
			return fmt.Errorf("failed to create temporary file for config: %w", err)
		}
		defer func() {
			if err := h.Configurer.DeleteFile(h, tempConfigPath); err != nil {
				log.Warnf("%s: failed to delete temporary file %s: %s", h, tempConfigPath, err)
			}
		}()

		if err := h.Configurer.WriteFile(h, tempConfigPath, cfgNew, "0600"); err != nil {
			return err
		}

		if err := p.validateConfig(h, tempConfigPath); err != nil {
			return err
		}

		cfgA := make(map[string]any)
		cfgB := make(map[string]any)
		if err := yaml.Unmarshal([]byte(cfgNew), &cfgA); err != nil {
			return fmt.Errorf("failed to unmarshal new config: %w", err)
		}
		if err := yaml.Unmarshal([]byte(h.Metadata.K0sExistingConfig), &cfgB); err != nil {
			return fmt.Errorf("failed to unmarshal existing config: %w", err)
		}
		cfgAString, err := yaml.Marshal(cfgA)
		if err != nil {
			return fmt.Errorf("failed to marshal new config: %w", err)
		}
		cfgBString, err := yaml.Marshal(cfgB)
		if err != nil {
			return fmt.Errorf("failed to marshal existing config: %w", err)
		}

		if bytes.Equal(cfgAString, cfgBString) {
			log.Debugf("%s: configuration will not change", h)
			continue
		}

		log.Debugf("%s: configuration will change", h)
		h.Metadata.K0sNewConfig = cfgNew
		p.hosts = append(p.hosts, h)
	}

	return nil
}

// DryRun prints the actions that would be taken
func (p *ConfigureK0s) DryRun() error {
	for _, h := range p.hosts {
		p.DryMsgf(h, "write k0s configuration to %s", h.Configurer.K0sConfigPath())
		switch p.configSource {
		case configSourceDefault:
			p.DryMsg(h, "k0s configuration is based on a generated k0s default configuration")
		case configSourceExisting:
			p.DryMsgf(h, "k0s configuration is based on an existing k0s configuration found on %s", p.Config.Spec.K0sLeader())
		case configSourceProvided:
			p.DryMsg(h, "k0s configuration is based on spec.k0s.config in k0sctl config")
		case configSourceNodeConfig:
			p.DryMsg(h, "k0s configuration is a generated node specific config for dynamic config clusters")
		}

		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(h.Metadata.K0sExistingConfig, h.Metadata.K0sNewConfig, false)
		p.DryMsgf(h, "configuration changes:\n%s", dmp.DiffPrettyText(diffs))

		if h.Metadata.K0sRunningVersion != nil && !h.Metadata.NeedsUpgrade {
			p.DryMsg(h, Colorize.BrightRed("restart the k0s service").String())
		}
	}
	return nil
}

// ShouldRun is true when there are controllers to configure
func (p *ConfigureK0s) ShouldRun() bool {
	return len(p.hosts) > 0
}

func (p *ConfigureK0s) generateDefaultConfig() (string, error) {
	log.Debugf("%s: generating default configuration", p.leader)
	var cmd string
	if configCreateSinceVersion.Check(p.leader.Metadata.K0sBinaryVersion) {
		cmd = p.leader.Configurer.K0sCmdf("config create --data-dir=%s", p.leader.K0sDataDir())
	} else {
		cmd = p.leader.Configurer.K0sCmdf("default-config")
	}

	cfg, err := p.leader.ExecOutput(cmd, exec.Sudo(p.leader))
	if err != nil {
		return "", err
	}

	return cfg, nil
}

// Run the phase
func (p *ConfigureK0s) Run() error {
	controllers := p.Config.Spec.Hosts.Controllers().Filter(func(h *cluster.Host) bool {
		return !h.Reset && len(h.Metadata.K0sNewConfig) > 0
	})
	return p.parallelDo(controllers, p.configureK0s)
}

func (p *ConfigureK0s) validateConfig(h *cluster.Host, configPath string) error {
	log.Infof("%s: validating configuration", h)

	var cmd string
	log.Debugf("%s: comparing k0s version %s with %s", h, p.Config.Spec.K0s.Version, configCreateSinceVersion)

	if h.Metadata.K0sBinaryTempFile != "" {
		oldK0sBinaryPath := h.Configurer.K0sBinaryPath()
		h.Configurer.SetPath("K0sBinaryPath", h.Metadata.K0sBinaryTempFile)
		defer func() {
			h.Configurer.SetPath("K0sBinaryPath", oldK0sBinaryPath)
		}()
	}

	if configCreateSinceVersion.Check(p.Config.Spec.K0s.Version) {
		log.Debugf("%s: comparison result true", h)
		cmd = h.Configurer.K0sCmdf(`config validate --config "%s"`, configPath)
	} else {
		log.Debugf("%s: comparison result false", h)
		cmd = h.Configurer.K0sCmdf(`validate config --config "%s"`, configPath)
	}

	var stderrBuf bytes.Buffer
	command, err := h.ExecStreams(cmd, nil, nil, &stderrBuf, exec.Sudo(h))
	if err != nil {
		return fmt.Errorf("can't run spec.k0s.config validation: %w", err)
	}
	if err := command.Wait(); err != nil {
		return fmt.Errorf("spec.k0s.config validation failed:: %w (%s)", err, stderrBuf.String())
	}

	return nil
}

func (p *ConfigureK0s) configureK0s(h *cluster.Host) error {
	path := h.K0sConfigPath()
	if h.Configurer.FileExist(h, path) {
		if !h.Configurer.FileContains(h, path, " generated-by-k0sctl") {
			newpath := path + ".old"
			log.Warnf("%s: an existing config was found and will be backed up as %s", h, newpath)
			if err := h.Configurer.MoveFile(h, path, newpath); err != nil {
				return err
			}
		}
	}

	log.Debugf("%s: writing k0s configuration", h)
	tempConfigPath, err := h.Configurer.TempFile(h)
	if err != nil {
		return fmt.Errorf("failed to create temporary file for config: %w", err)
	}

	if err := h.Configurer.WriteFile(h, tempConfigPath, h.Metadata.K0sNewConfig, "0600"); err != nil {
		return err
	}

	log.Infof("%s: installing new configuration", h)
	configPath := h.K0sConfigPath()
	configDir := gopath.Dir(configPath)

	if !h.Configurer.FileExist(h, configDir) {
		if err := h.Execf(`install -m 0750 -o root -g root -d "%s"`, configDir, exec.Sudo(h)); err != nil {
			return fmt.Errorf("failed to create k0s configuration directory: %w", err)
		}
	}

	if err := h.Execf(`install -m 0600 -o root -g root "%s" "%s"`, tempConfigPath, configPath, exec.Sudo(h)); err != nil {
		return fmt.Errorf("failed to install k0s configuration: %w", err)
	}

	if h.Metadata.K0sRunningVersion != nil && !h.Metadata.NeedsUpgrade {
		log.Infof("%s: restarting k0s service", h)
		if err := h.Configurer.RestartService(h, h.K0sServiceName()); err != nil {
			return err
		}

		log.Infof("%s: waiting for k0s service to start", h)
		return retry.Timeout(context.TODO(), retry.DefaultTimeout, node.ServiceRunningFunc(h, h.K0sServiceName()))
	}

	return nil
}

func (p *ConfigureK0s) configFor(h *cluster.Host) (string, error) {
	var cfg dig.Mapping

	if p.Config.Spec.K0s.DynamicConfig {
		if h == p.leader && h.Metadata.K0sRunningVersion == nil {
			log.Debugf("%s: leader will get a full config on initialize ", h)
			cfg = p.newBaseConfig.Dup()
		} else {
			log.Debugf("%s: using a stripped down config for dynamic config", h)
			cfg = p.Config.Spec.K0s.NodeConfig()
		}
	} else {
		cfg = p.newBaseConfig.Dup()
	}

	var (
		sans []string
		addr string
	)

	if h.PrivateAddress != "" {
		addr = h.PrivateAddress
	} else {
		addr = h.Address()
	}

	// convert sans from unmarshaled config into []string
	oldsans := cfg.Dig("spec", "api", "sans")
	switch oldsans := oldsans.(type) {
	case []interface{}:
		for _, v := range oldsans {
			if s, ok := v.(string); ok {
				sans = append(sans, s)
			}
		}
	case []string:
		sans = append(sans, oldsans...)
	}

	if !slices.Contains(sans, addr) {
		sans = append(sans, addr)
		log.Infof("%s: added %s to spec.api.sans", h, addr)
	}

	for i, c := range p.Config.Spec.Hosts.Controllers() {
		if !slices.Contains(sans, c.Address()) {
			sans = append(sans, c.Address())
			log.Infof("%s: added controller %d address %s to spec.api.sans", h, i+1, c.Address())
		}
		if c.PrivateAddress != "" && !slices.Contains(sans, c.PrivateAddress) {
			sans = append(sans, c.PrivateAddress)
			log.Infof("%s: added controller %d private address %s to spec.api.sans", h, i+1, c.PrivateAddress)
		}
	}

	log.Debugf("%s: using k0s spec.api.sans: %v", h, sans)
	cfg.DigMapping("spec", "api")["sans"] = sans

	if cfg.DigString("spec", "api", "address") == "" {
		if onlyBindAddr, ok := cfg.Dig("spec", "api", "onlyBindToAddress").(bool); ok && onlyBindAddr {
			cfg.DigMapping("spec", "api")["address"] = addr
		}
	}

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
