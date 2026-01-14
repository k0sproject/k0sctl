package phase

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
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
		p.validateOS,
		p.warnK0sBinaryPath,
		p.validateUniqueHostname,
		p.validateUniqueMachineID,
		p.validateUniquePrivateAddress,
		p.validateSudo,
		p.validateConfigurer,
		p.cleanUpOldK0sTmpFiles,
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

func (p *ValidateHosts) validateConfigurer(_ context.Context, h *cluster.Host) error {
	validator, ok := h.Configurer.(configurer.HostValidator)
	if !ok {
		return nil
	}

	return validator.ValidateHost(h)
}

var k0sWindowsWorkerSupportSince = version.MustConstraint(">= 1.34.0-0")

func (p *ValidateHosts) validateOS(_ context.Context, h *cluster.Host) error {
	if !h.IsWindows() || p.Config.Spec.K0s.Version == nil {
		return nil
	}

	if h.IsController() {
		return fmt.Errorf("windows is not supported on k0s controller nodes")
	}

	if !k0sWindowsWorkerSupportSince.Check(p.Config.Spec.K0s.Version) {
		return fmt.Errorf("windows workers require k0s version %s", k0sWindowsWorkerSupportSince)
	}

	log.Warnf("%s: windows worker node support is experimental", h)
	return nil
}

const cleanUpOlderThan = 30 * time.Minute

// clean up any k0s.tmp.* files from K0sBinaryPath that are older than 30 minutes and warn if there are any that are newer than that
func (p *ValidateHosts) cleanUpOldK0sTmpFiles(_ context.Context, h *cluster.Host) error {
	err := fs.WalkDir(h.SudoFsys(), filepath.Join(filepath.Dir(h.K0sInstallLocation()), "k0s.tmp.*"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Warnf("failed to walk k0s.tmp.* files in %s: %v", h.K0sInstallLocation(), err)
			return nil
		}
		log.Debugf("%s: found k0s binary upload temporary file %s", h, path)
		info, err := d.Info()
		if err != nil {
			log.Warnf("%s: failed to get info for %s: %v", h, path, err)
			return nil
		}
		if time.Since(info.ModTime()) > cleanUpOlderThan {
			log.Warnf("%s: cleaning up old k0s binary upload temporary file %s", h, path)
			if err := h.Configurer.DeleteFile(h, path); err != nil {
				log.Warnf("%s: failed to delete %s: %v", h, path, err)
			}
			return nil
		}
		log.Warnf("%s: found k0s binary upload temporary file %s that is newer than %s", h, path, cleanUpOlderThan)
		return nil
	})
	if err != nil {
		log.Warnf("failed to walk k0s.tmp.* files in %s: %v", h.K0sInstallLocation(), err)
	}
	return nil
}

const maxSkew = 30 * time.Second

func (p *ValidateHosts) validateClockSkew(ctx context.Context) error {
	log.Infof("validating clock skew")
	skews := make(map[*cluster.Host]time.Duration, len(p.Config.Spec.Hosts))
	var skewValues []time.Duration
	var mu sync.Mutex

	// Collect skews relative to local time
	err := p.parallelDo(ctx, p.Config.Spec.Hosts, func(_ context.Context, h *cluster.Host) error {
		remote, err := h.Configurer.SystemTime(h)
		if err != nil {
			return fmt.Errorf("failed to get time from %s: %w", h, err)
		}
		skew := time.Now().UTC().Sub(remote).Round(time.Second)
		mu.Lock()
		skews[h] = skew
		skewValues = append(skewValues, skew)
		mu.Unlock()
		return nil
	})
	if err != nil {
		return err
	}

	// Sort skews to find the median
	slices.Sort(skewValues)
	median := skewValues[len(skewValues)/2]

	// Check if any skew exceeds the maxSkew relative to the median
	var foundExceeding int
	for h, skew := range skews {
		deviation := (skew - median).Abs()
		if deviation > maxSkew {
			log.Errorf("%s: clock skew of %.0f seconds exceeds the maximum of %.0f seconds", h, deviation.Seconds(), maxSkew.Seconds())
			foundExceeding++
		}
	}

	if foundExceeding > 0 {
		return fmt.Errorf("clock skew exceeds the maximum on %d hosts", foundExceeding)
	}

	return nil
}
