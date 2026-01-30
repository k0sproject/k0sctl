package phase

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// Note: Passwordless sudo has not yet been confirmed when this runs

// GatherFacts gathers information about hosts, such as if k0s is already up and running
type GatherFacts struct {
	GenericPhase
	SkipMachineIDs bool
}

var (
	// K0s doesn't rely on unique machine IDs anymore since v1.30.
	uniqueMachineIDSince = version.MustParse("v1.30.0")
	// --kubelet-root-dir was introduced in v1.32.1-rc.0
	kubeletRootDirSince = version.MustParse("v1.32.1-rc.0")
)

// Title for the phase
func (p *GatherFacts) Title() string {
	return "Gather host facts"
}

// Run the phase
func (p *GatherFacts) Run(ctx context.Context) error {
	return p.parallelDo(ctx, p.Config.Spec.Hosts, p.investigateHost)
}

func (p *GatherFacts) investigateHost(_ context.Context, h *cluster.Host) error {
	arch, err := h.Arch()
	if err != nil {
		return err
	}
	log.Infof("%s: detected %s architecture", h, arch)

	if !p.SkipMachineIDs && p.Config.Spec.K0s.Version.LessThan(uniqueMachineIDSince) {
		id, err := h.Configurer.MachineID(h)
		if err != nil {
			return err
		}
		h.Metadata.MachineID = id
	}

	if extra := h.InstallFlags.GetValue("--kubelet-extra-args"); extra != "" {
		ef := cluster.Flags{extra}
		if over := ef.GetValue("--hostname-override"); over != "" {
			if h.HostnameOverride != "" && h.HostnameOverride != over {
				return fmt.Errorf("hostname and installFlags kubelet-extra-args hostname-override mismatch, only define either one")
			}
			h.HostnameOverride = over
		}
	}

	if h.HostnameOverride != "" {
		log.Infof("%s: using %s from configuration as hostname", h, h.HostnameOverride)
		h.Metadata.Hostname = h.HostnameOverride
	} else {
		n := h.Configurer.Hostname(h)
		if n == "" {
			return fmt.Errorf("%s: failed to resolve a hostname", h)
		}
		h.Metadata.Hostname = n
		log.Infof("%s: using %s as hostname", h, n)
	}

	if h.PrivateAddress == "" {
		if h.PrivateInterface == "" {
			if iface, err := h.Configurer.PrivateInterface(h); err == nil {
				h.PrivateInterface = iface
				log.Infof("%s: discovered %s as private interface", h, iface)
			}
		}

		if h.PrivateInterface != "" {
			if addr, err := h.Configurer.PrivateAddress(h, h.PrivateInterface, h.Address()); err == nil {
				h.PrivateAddress = addr
				log.Infof("%s: discovered %s as private address", h, addr)
			}
		}
	}

	if p.Config.Spec.K0s.Version.LessThan(kubeletRootDirSince) && h.KubeletRootDir != "" {
		return fmt.Errorf("kubeletRootDir is not supported in k0s version %s, please remove it from the configuration", p.Config.Spec.K0s.Version)
	}

	if h.UseExistingK0s {
		if h.K0sBinaryPath == "" {
			path, err := h.Configurer.LookPath(h, "k0s")
			if err != nil {
				return fmt.Errorf("%s: useExistingK0s=true but no 'k0s' binary found in PATH, set k0sInstallPath to use a custom path", h)
			}
			log.Infof("%s: found existing 'k0s' binary at %s", h, path)
			h.K0sInstallPath = path
			h.Configurer.SetPath("K0sBinaryPath", path)
		} else if !h.Configurer.FileExist(h, h.K0sBinaryPath) {
			return fmt.Errorf("%s: useExistingK0s=true but no 'k0s' binary found at %s", h, h.K0sBinaryPath)
		}
	}

	return nil
}
