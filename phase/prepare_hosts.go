package phase

import (
	"errors"
	"strings"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/os"
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
	return p.Config.Spec.Hosts.ParallelEach(p.prepareHost)
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

	err := retry.Do(
		func() error {
			return h.Configurer.TryLock(h)
		},
		retry.OnRetry(
			func(n uint, err error) {
				log.Errorf("%s: attempt %d of %d.. trying to obtain a lock on host: %s", h, n+1, retries, err.Error())
			},
		),
		retry.RetryIf(
			func(err error) bool {
				return !strings.Contains(err.Error(), "host does not have")
			},
		),
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.MaxJitter(time.Second*2),
		retry.Delay(time.Second*3),
		retry.Attempts(5),
		retry.LastErrorOnly(true),
	)

	if err != nil && !strings.Contains(err.Error(), "host does not have") {
		return errors.New("another k0sctl instance is currently operating on the node")
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

	if h.NeedIPTables() {
		pkgs = append(pkgs, "iptables")
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
