package phase

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type k0sstatus struct {
	Version       *version.Version `json:"Version"`
	Pid           int              `json:"Pid"`
	PPid          int              `json:"PPid"`
	Role          string           `json:"Role"`
	SysInit       string           `json:"SysInit"`
	StubFile      string           `json:"StubFile"`
	Workloads     bool             `json:"Workloads"`
	Args          []string         `json:"Args"`
	ClusterConfig dig.Mapping      `json:"ClusterConfig"`
	K0sVars       dig.Mapping      `json:"K0sVars"`
}

func (k *k0sstatus) isSingle() bool {
	for _, a := range k.Args {
		if a == "--single=true" {
			return true
		}
	}
	return false
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
	if err := p.parallelDo(controllers, p.investigateK0s); err != nil {
		return err
	}
	p.leader = p.Config.Spec.K0sLeader()

	if id, err := p.Config.Spec.K0s.GetClusterID(p.leader); err == nil {
		p.Config.Spec.K0s.Metadata.ClusterID = id
		p.SetProp("clusterID", id)
	}

	var workers cluster.Hosts = p.Config.Spec.Hosts.Workers()
	if err := p.parallelDo(workers, p.investigateK0s); err != nil {
		return err
	}

	return nil
}

func (p *GatherK0sFacts) investigateK0s(h *cluster.Host) error {
	output, err := h.ExecOutput(h.Configurer.K0sCmdf("version"), exec.Sudo(h))
	if err != nil {
		log.Debugf("%s: no 'k0s' binary in PATH", h)
		return nil
	}

	binVersion, err := version.NewVersion(strings.TrimSpace(output))
	if err != nil {
		return fmt.Errorf("failed to parse installed k0s version: %w", err)
	}

	h.Metadata.K0sBinaryVersion = binVersion

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

	var existingServiceScript string

	for _, svc := range []string{"k0scontroller", "k0sworker", "k0sserver"} {
		if path, err := h.Configurer.ServiceScriptPath(h, svc); err == nil && path != "" {
			existingServiceScript = path
			break
		}
	}

	output, err = h.ExecOutput(h.Configurer.K0sCmdf("status -o json"), exec.Sudo(h))
	if err != nil {
		if existingServiceScript == "" {
			log.Debugf("%s: an existing k0s instance is not running and does not seem to have been installed as a service", h)
			return nil
		}

		if Force {
			log.Warnf("%s: an existing k0s instance is not running but has been installed as a service at %s - ignoring because --force was given", h, existingServiceScript)
			return nil
		}

		return fmt.Errorf("k0s doesn't appear to be running but has been installed as a service at %s - please remove it or start the service", existingServiceScript)
	}

	if existingServiceScript == "" {
		return fmt.Errorf("k0s is running but has not been installed as a service, possibly a non-k0sctl managed host or a broken installation - you can try to reset the host by setting `reset: true` on it")
	}

	status := k0sstatus{}

	if err := json.Unmarshal([]byte(output), &status); err != nil {
		log.Warnf("%s: failed to decode k0s status output: %s", h, err.Error())
		return nil
	}

	if status.Version == nil || status.Role == "" || status.Pid == 0 {
		log.Debugf("%s: k0s is not running", h)
		return nil
	}

	switch status.Role {
	case "server":
		status.Role = "controller"
	case "server+worker":
		status.Role = "controller+worker"
	case "controller":
		if status.Workloads {
			if status.isSingle() {
				status.Role = "single"
			} else {
				status.Role = "controller+worker"
			}
		}
	}

	if status.Role != h.Role {
		return fmt.Errorf("%s: is configured as k0s %s but is already running as %s - role change is not supported", h, h.Role, status.Role)
	}

	h.Metadata.K0sRunningVersion = status.Version
	h.Metadata.NeedsUpgrade = p.needsUpgrade(h)

	log.Infof("%s: is running k0s %s version %s", h, h.Role, h.Metadata.K0sRunningVersion)
	if h.IsController() {
		for _, a := range status.Args {
			if strings.HasPrefix(a, "--enable-dynamic-config") && !strings.HasSuffix(a, "false") {
				if !p.Config.Spec.K0s.DynamicConfig {
					log.Warnf("%s: controller has dynamic config enabled, but spec.k0s.dynamicConfig was not set in configuration, proceeding in dynamic config mode", h)
					p.Config.Spec.K0s.DynamicConfig = true
				}
			}
		}
		if h.InstallFlags.Include("--enable-dynamic-config") {
			if val := h.InstallFlags.GetValue("--enable-dynamic-config"); val != "false" {
				if !p.Config.Spec.K0s.DynamicConfig {
					log.Warnf("%s: controller has --enable-dynamic-config in installFlags, but spec.k0s.dynamicConfig was not set in configuration, proceeding in dynamic config mode", h)
				}
				p.Config.Spec.K0s.DynamicConfig = true
			}
		}

		if p.Config.Spec.K0s.DynamicConfig {
			h.InstallFlags.AddOrReplace("--enable-dynamic-config")
		}
	}

	if h.Role == "controller+worker" && !h.NoTaints {
		log.Warnf("%s: the controller+worker node will not schedule regular workloads without toleration for node-role.kubernetes.io/master:NoSchedule unless 'noTaints: true' is set", h)
	}

	if h.Metadata.NeedsUpgrade {
		log.Warnf("%s: k0s will be upgraded", h)
	}

	if !h.IsController() {
		log.Infof("%s: checking if worker %s has joined", p.leader, h.Metadata.Hostname)
		if err := node.KubeNodeReadyFunc(h)(context.Background()); err != nil {
			log.Debugf("%s: failed to get ready status: %s", h, err.Error())
		} else {
			h.Metadata.Ready = true
		}
	}

	return nil
}

func (p *GatherK0sFacts) needsUpgrade(h *cluster.Host) bool {
	// If supplimental files or a k0s binary have been specified explicitly,
	// always upgrade.  This covers the scenario where a user moves from a
	// default-install cluster to one fed by OCI image bundles (ie. airgap)
	for _, f := range h.Files {
		if f.IsURL() {
			log.Debugf("%s: marked for upgrade because there are URL source file uploads for the host", h)
			return true
		}

		for _, s := range f.Sources {
			dest := f.DestinationFile
			if dest == "" {
				dest = path.Join(f.DestinationDir, s.Path)
			}
			src := path.Join(f.Base, s.Path)

			if h.FileChanged(src, dest) {
				log.Debugf("%s: marked for upgrade because file was changed for upload %s", h, src)
				return true
			}
		}
	}

	if h.K0sBinaryPath != "" && h.FileChanged(h.K0sBinaryPath, h.Configurer.K0sBinaryPath()) {
		log.Debugf("%s: marked for upgrade because of a static k0s binary path %s", h, h.K0sBinaryPath)
		return true
	}

	return p.Config.Spec.K0s.Version.GreaterThan(h.Metadata.K0sRunningVersion)
}
