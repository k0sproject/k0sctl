package phase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
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
	hosts  cluster.Hosts
}

// Title for the phase
func (p *GatherK0sFacts) Title() string {
	return "Gather k0s facts"
}

// Prepare finds hosts with k0s installed
func (p *GatherK0sFacts) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.Exec(h.Configurer.K0sCmdf("version"), exec.Sudo(h)) == nil
	})

	return nil
}

// ShouldRun is true when there are hosts that need to be connected
func (p *GatherK0sFacts) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *GatherK0sFacts) Run(ctx context.Context) error {
	controllers := p.hosts.Controllers()
	if err := p.parallelDo(ctx, controllers, p.investigateK0s); err != nil {
		return err
	}
	p.leader = p.Config.Spec.K0sLeader()
	p.leader.Metadata.IsK0sLeader = true

	if id, err := p.Config.Spec.K0s.GetClusterID(p.leader); err == nil {
		p.Config.Spec.K0s.Metadata.ClusterID = id
	}

	if err := p.investigateEtcd(); err != nil {
		return err
	}

	workers := p.hosts.Workers()
	if err := p.parallelDo(ctx, workers, p.investigateK0s); err != nil {
		return err
	}

	return nil
}

func (p *GatherK0sFacts) isInternalEtcd() bool {
	if p.leader.Role != "controller" && p.leader.Role != "controller+worker" {
		return false
	}

	if p.leader.Metadata.K0sRunningVersion == nil {
		return false
	}

	if p.Config.Spec.K0s == nil || p.Config.Spec.K0s.Config == nil {
		log.Debugf("%s: k0s config not found, expecting default internal etcd", p.leader)
		return true
	}

	log.Debugf("%s: checking storage config for etcd", p.leader)
	if storageConfig, ok := p.Config.Spec.K0s.Config.Dig("spec", "storage").(dig.Mapping); ok {
		storageType := storageConfig.DigString("type")
		switch storageType {
		case "etcd":
			if _, ok := storageConfig.Dig("etcd", "externalCluster").(dig.Mapping); ok {
				log.Debugf("%s: storage is configured with external etcd", p.leader)
				return false
			}
			log.Debugf("%s: storage type is etcd", p.leader)
			return true
		case "":
			log.Debugf("%s: storage type is default", p.leader)
			return true
		default:
			log.Debugf("%s: storage type is %s", p.leader, storageType)
			return false
		}
	}

	log.Debugf("%s: storage config not found, expecting default internal etcd", p.leader)
	return true
}

func (p *GatherK0sFacts) investigateEtcd() error {
	if !p.isInternalEtcd() {
		log.Debugf("%s: skipping etcd member list", p.leader)
		return nil
	}

	if err := p.listEtcdMembers(p.leader); err != nil {
		return err
	}

	return nil
}

func (p *GatherK0sFacts) listEtcdMembers(h *cluster.Host) error {
	log.Infof("%s: listing etcd members", h)
	// etcd member-list outputs json like:
	// {"members":{"controller0":"https://172.17.0.2:2380","controller1":"https://172.17.0.3:2380"}}
	// on versions like ~1.21.x etcd member-list outputs to stderr with extra fields (from logrus).

	// Sometimes, random log statements may appear on stderr, which can break
	// the parsing. Try to parse stdout first, then fallback to stderr. Error
	// out if none of both was a JSON document.

	var stdout, stderr bytes.Buffer
	if cmd, err := h.ExecStreams(
		h.Configurer.K0sCmdf("etcd member-list --data-dir=%s", h.K0sDataDir()),
		nil /*stdin*/, &stdout, &stderr,
		exec.Sudo(h),
	); err != nil {
		return fmt.Errorf("failed to create etcd member-list command: %w", err)
	} else if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to run etcd member-list command: %w", err)
	}

	var (
		result map[string]any
		errs   []error
	)
	for _, output := range [][]byte{stdout.Bytes(), stderr.Bytes()} {
		if len(output) < 1 {
			continue
		}
		unmarshalled := make(map[string]any)
		err := json.Unmarshal(output, &unmarshalled)
		if err == nil {
			result = unmarshalled
			break
		}
		errs = append(errs, err)
	}
	if result == nil {
		err := errors.Join(errs...)
		if err == nil {
			err = errors.New("no data")
		}
		return fmt.Errorf("failed to decode etcd member-list output: %w", err)
	}

	etcdMembers := []string{}
	if members, ok := result["members"].(map[string]any); ok {
		for _, urlField := range members {
			urlFieldStr, ok := urlField.(string)
			if ok {
				memberURL, err := url.Parse(urlFieldStr)
				if err != nil {
					return fmt.Errorf("failed to parse etcd member URL: %w", err)
				}
				memberHost, _, err := net.SplitHostPort(memberURL.Host)
				if err != nil {
					return fmt.Errorf("failed to split etcd member URL: %w", err)
				}
				log.Debugf("%s: detected etcd member %s", h, memberHost)
				etcdMembers = append(etcdMembers, memberHost)
			}
		}
	}

	p.Config.Metadata.EtcdMembers = etcdMembers
	return nil
}

func (p *GatherK0sFacts) investigateK0s(ctx context.Context, h *cluster.Host) error {
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

	if h.IsController() && h.Configurer.FileExist(h, h.K0sConfigPath()) {
		cfg, err := h.Configurer.ReadFile(h, h.K0sConfigPath())
		if cfg != "" && err == nil {
			log.Infof("%s: found existing configuration", h)
			h.Metadata.K0sExistingConfig = cfg
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
	if p.Config.Spec.K0s.Version == nil {
		p.Config.Spec.K0s.Version = status.Version
	}

	h.Metadata.NeedsUpgrade = p.needsUpgrade(h)

	var args cluster.Flags
	if len(status.Args) > 2 {
		// status.Args contains the binary path and the role as the first two elements, which we can ignore here.
		for _, a := range status.Args[2:] {
			args.Add(a)
		}
	}
	h.Metadata.K0sStatusArgs = args

	log.Infof("%s: is running k0s %s version %s", h, h.Role, h.Metadata.K0sRunningVersion)
	if h.IsController() {
		for _, a := range h.Metadata.K0sStatusArgs {
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
		if err := node.KubeNodeReadyFunc(h)(ctx); err != nil {
			log.Debugf("%s: failed to get ready status: %s", h, err.Error())
		} else {
			h.Metadata.Ready = true
		}
	}

	return nil
}

func (p *GatherK0sFacts) needsUpgrade(h *cluster.Host) bool {
	if h.Reset {
		return false
	}

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
