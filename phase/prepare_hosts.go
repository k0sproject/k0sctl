package phase

import (
	"strings"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// PrepareHosts installs required packages and so on on the hosts.
type PrepareHosts struct {
	GenericPhase
}

// Title for the phase
func (p *PrepareHosts) Title() string {
	return "Prepare hosts"
}

// Run the phase
func (p *PrepareHosts) Run() error {
	return p.parallelDo(p.Config.Spec.Hosts, p.prepareHost)
}

type prepare interface {
	Prepare(os.Host) error
}

func (p *PrepareHosts) prepareHost(h *cluster.Host) error {
	if c, ok := h.Configurer.(prepare); ok {
		if err := c.Prepare(h); err != nil {
			return err
		}
	}

	if len(h.Environment) > 0 {
		log.Infof("%s: updating environment", h)
		if err := h.Configurer.UpdateEnvironment(h, h.Environment); err != nil {
			return err
		}
	}

	var pkgs []string

	if h.NeedCurl() {
		pkgs = append(pkgs, "curl")
	}

	// iptables is only required for very old versions of k0s
	if k0sVer, err := version.NewVersion(p.Config.Spec.K0s.Version); err == nil {
		if thresholdVersion, err := version.NewVersion("v1.22.1+k0s.0"); err != nil {
			panic(err)
		} else if k0sVer.LessThan(thresholdVersion) && h.NeedIPTables() { //nolint:staticcheck
			pkgs = append(pkgs, "iptables")
		}
	}

	if h.NeedInetUtils() {
		pkgs = append(pkgs, "inetutils")
	}

	if len(pkgs) > 0 {
		log.Infof("%s: installing packages (%s)", h, strings.Join(pkgs, ", "))
		if err := h.Configurer.InstallPackage(h, pkgs...); err != nil {
			return err
		}
	}

	if h.Configurer.IsContainer(h) {
		log.Infof("%s: is a container, applying a fix", h)
		if err := h.Configurer.FixContainer(h); err != nil {
			return err
		}
	}

	return nil
}
