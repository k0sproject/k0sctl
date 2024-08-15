package phase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/os"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

var iptablesEmbeddedSince = version.MustConstraint(">= v1.22.1+k0s.0")

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

// updateEnvironment updates the environment variables on the host and reconnects to
// it if necessary.
func (p *PrepareHosts) updateEnvironment(h *cluster.Host) error {
	if err := h.Configurer.UpdateEnvironment(h, h.Environment); err != nil {
		return err
	}
	if h.Connection.Protocol() != "SSH" {
		return nil
	}
	// XXX: this is a workaround. UpdateEnvironment on rig's os/linux.go writes
	// the environment to /etc/environment and then exports the same variables
	// using 'export' command. This is not enough for the environment to be
	// preserved across multiple ssh sessions. We need to write the environment
	// and then reopen the ssh session. Go's ssh client.Setenv() depends on ssh
	// server configuration (sshd only accepts LC_* variables by default).
	log.Infof("%s: reconnecting to apply new environment", h)
	h.Disconnect()
	return retry.Timeout(context.TODO(), 10*time.Minute, func(_ context.Context) error {
		if err := h.Connect(); err != nil {
			if errors.Is(err, rig.ErrCantConnect) || strings.Contains(err.Error(), "host key mismatch") {
				return errors.Join(retry.ErrAbort, err)
			}
			return fmt.Errorf("failed to reconnect to %s: %w", h, err)
		}
		return nil
	})
}

func (p *PrepareHosts) prepareHost(h *cluster.Host) error {
	if c, ok := h.Configurer.(prepare); ok {
		if err := c.Prepare(h); err != nil {
			return err
		}
	}

	if len(h.Environment) > 0 {
		log.Infof("%s: updating environment", h)
		if err := p.updateEnvironment(h); err != nil {
			return fmt.Errorf("failed to updated environment: %w", err)
		}
	}

	var pkgs []string

	if h.NeedCurl() {
		pkgs = append(pkgs, "curl")
	}

	// iptables is only required for very old versions of k0s
	if p.Config.Spec.K0s.Version != nil && !iptablesEmbeddedSince.Check(p.Config.Spec.K0s.Version) && h.NeedIPTables() { //nolint:staticcheck
		pkgs = append(pkgs, "iptables")
	}

	if h.NeedInetUtils() {
		pkgs = append(pkgs, "inetutils")
	}

	for _, pkg := range pkgs {
		err := p.Wet(h, fmt.Sprintf("install package %s", pkg), func() error {
			log.Infof("%s: installing package %s", h, pkg)
			return h.Configurer.InstallPackage(h, pkg)
		})
		if err != nil {
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
