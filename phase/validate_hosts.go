package phase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	log "github.com/sirupsen/logrus"
)

// ValidateHosts performs remote OS detection
type ValidateHosts struct {
	GenericPhase
	hncount          map[string]int
	machineidcount   map[string]int
	privateaddrcount map[string]int
}

// Title for the phase
func (p *ValidateHosts) Title() string {
	return "Validate hosts"
}

// Run the phase
func (p *ValidateHosts) Run(ctx context.Context) error {
	p.hncount = make(map[string]int, len(p.Config.Spec.Hosts))
	if p.Config.Spec.K0s.Version.LessThan(uniqueMachineIDSince) {
		p.machineidcount = make(map[string]int, len(p.Config.Spec.Hosts))
	}
	p.privateaddrcount = make(map[string]int, len(p.Config.Spec.Hosts))

	controllerCount := len(p.Config.Spec.Hosts.Controllers())
	var resetControllerCount int
	for _, h := range p.Config.Spec.Hosts {
		p.hncount[h.Metadata.Hostname]++
		if p.machineidcount != nil {
			p.machineidcount[h.Metadata.MachineID]++
		}
		if h.PrivateAddress != "" {
			p.privateaddrcount[h.PrivateAddress]++
		}
		if h.IsController() && h.Reset {
			resetControllerCount++
		}
	}

	if resetControllerCount >= controllerCount {
		return fmt.Errorf("all controllers are marked to be reset - this will break the cluster. use `k0sctl reset` instead if that is intentional")
	}

	err := p.parallelDo(
		ctx,
		p.Config.Spec.Hosts,
		p.warnK0sBinaryPath,
		p.validateUniqueHostname,
		p.validateUniqueMachineID,
		p.validateUniquePrivateAddress,
		p.validateSudo,
	)
	if err != nil {
		return err
	}
	return p.validateClockSkew(ctx)
}

func (p *ValidateHosts) warnK0sBinaryPath(_ context.Context, h *cluster.Host) error {
	if h.K0sBinaryPath != "" {
		log.Warnf("%s: k0s binary path is set to %q, version checking for the host is disabled. The k0s version for other hosts is %s.", h, h.K0sBinaryPath, p.Config.Spec.K0s.Version)
	}

	return nil
}

func (p *ValidateHosts) validateUniqueHostname(_ context.Context, h *cluster.Host) error {
	if p.hncount[h.Metadata.Hostname] > 1 {
		return fmt.Errorf("hostname is not unique: %s", h.Metadata.Hostname)
	}

	return nil
}

func (p *ValidateHosts) validateUniquePrivateAddress(_ context.Context, h *cluster.Host) error {
	if p.privateaddrcount[h.PrivateAddress] > 1 {
		return fmt.Errorf("privateAddress %q is not unique: %s", h.PrivateAddress, h.Metadata.Hostname)
	}

	return nil
}

func (p *ValidateHosts) validateUniqueMachineID(_ context.Context, h *cluster.Host) error {
	if p.machineidcount[h.Metadata.MachineID] > 1 {
		return fmt.Errorf("machine id %s is not unique: %s", h.Metadata.MachineID, h.Metadata.Hostname)
	}

	return nil
}

func (p *ValidateHosts) validateSudo(_ context.Context, h *cluster.Host) error {
	if err := h.Configurer.CheckPrivilege(h); err != nil {
		return err
	}

	return nil
}

const maxSkew = 30 * time.Second

func (p *ValidateHosts) validateClockSkew(ctx context.Context) error {
	log.Infof("validating clock skew")
	skews := make(map[*cluster.Host]time.Duration, len(p.Config.Spec.Hosts))
	var mu sync.Mutex
	err := p.parallelDo(ctx, p.Config.Spec.Hosts, func(_ context.Context, h *cluster.Host) error {
		remote, err := h.Configurer.SystemTime(h)
		if err != nil {
			return fmt.Errorf("failed to get time from %s: %w", h, err)
		}
		mu.Lock()
		skews[h] = time.Now().UTC().Sub(remote).Round(time.Second).Abs()
		mu.Unlock()
		return nil
	})
	if err != nil {
		return err
	}

	// find maximum deviation
	var max time.Duration
	var maxHost *cluster.Host
	for h, skew := range skews {
		abs := skew.Abs()
		if abs > max {
			max = abs
			maxHost = h
		}
	}

	// find deviations from the maximum
	var foundExceeding int
	for h, skew := range skews {
		if h == maxHost {
			continue
		}
		deviation := skew.Abs() - max
		log.Debugf("%s: clock skew compared to highest is %.0f seconds", h, skew.Seconds())
		if deviation > maxSkew {
			log.Errorf("%s: clock skew of %.0f seconds exceeds the maximum of %.0f", h, skew.Seconds(), maxSkew.Seconds())
			foundExceeding++
		}
	}

	if foundExceeding > 0 {
		return fmt.Errorf("clock skew exceeds the maximum on %d hosts", foundExceeding)
	}
	return nil
}
