package phase

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	k0s "github.com/k0sproject/k0sctl/pkg/k0s"
	log "github.com/sirupsen/logrus"
)

// StageBinaries stages k0s binaries on hosts that need them using the host's configured BinaryProvider.
type StageBinaries struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *StageBinaries) Title() string {
	return "Stage k0s binaries on hosts"
}

// Prepare the phase
func (p *StageBinaries) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	var prepareErr error
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		if h.Reset {
			log.Debugf("%s: skipping binary staging (reset)", h)
			return false
		}
		provider, err := h.K0sBinaryProvider(p.Config.Spec.K0s.Version)
		if err != nil {
			if prepareErr == nil {
				prepareErr = fmt.Errorf("%s: %w", h, err)
			}
			return false
		}
		metaNeeds := h.Metadata.NeedsUpgrade
		providerNeeds := provider.NeedsUpgrade()
		if providerNeeds {
			log.Debugf("%s: will stage binary via %T (metaNeedsUpgrade=%v, providerNeedsUpgrade=%v)", h, provider, metaNeeds, providerNeeds)
			return true
		}
		log.Debugf("%s: binary staging not needed (metaNeedsUpgrade=%v, providerNeedsUpgrade=%v)", h, metaNeeds, providerNeeds)
		return false
	})
	return prepareErr
}

// ShouldRun is true when there are hosts that need binary staging
func (p *StageBinaries) ShouldRun() bool {
	return len(p.hosts) > 0
}

// DryRun stages binaries in dry-run mode the same as in wet mode, because
// subsequent phases (e.g. config validate) need the staged binary for validation.
// No DryMsg/Wet messages are emitted here: staging is a local/temporary side effect,
// not a permanent cluster change — on success, the temp file is removed by
// Disconnect (including its DryRun behavior), while CleanUp is only invoked on
// failure paths. CleanUp is also called here on error as an extra safety net.
func (p *StageBinaries) DryRun() error {
	if err := p.Run(context.Background()); err != nil {
		p.CleanUp()
		return err
	}
	return nil
}

// Run the phase
func (p *StageBinaries) Run(ctx context.Context) error {
	var uploadHosts, otherHosts cluster.Hosts
	for _, h := range p.hosts {
		provider, err := h.K0sBinaryProvider(p.Config.Spec.K0s.Version)
		if err != nil {
			return fmt.Errorf("%s: %w", h, err)
		}
		if provider.IsUpload() {
			uploadHosts = append(uploadHosts, h)
		} else {
			otherHosts = append(otherHosts, h)
		}
	}

	// Populate local caches sequentially, deduplicated by cache key, so each
	// unique binary is downloaded at most once regardless of how many hosts need it.
	if err := p.populateCaches(ctx, uploadHosts); err != nil {
		return err
	}

	// Stage both groups sequentially so the combined concurrency never exceeds
	// the global limit, while still respecting the upload concurrency sub-limit.
	if err := p.parallelDo(ctx, otherHosts, p.stageForHost); err != nil {
		return err
	}
	return p.parallelDoUpload(ctx, uploadHosts, p.stageForHost)
}

// stageForHost stages the k0s binary on the host even in dry-run mode, so that
// subsequent phases can use the staged binary for validation (e.g. config validate)
// without installing it. The binary is cleaned up by Disconnect.DryRun on exit.
func (p *StageBinaries) stageForHost(ctx context.Context, h *cluster.Host) error {
	provider, err := h.K0sBinaryProvider(p.Config.Spec.K0s.Version)
	if err != nil {
		return err
	}
	log.Debugf("%s: staging k0s binary using %T", h, provider)
	tmp, err := provider.Stage(ctx)
	if err != nil {
		return err
	}
	if tmp != "" {
		log.Debugf("%s: staged k0s binary to %s", h, tmp)
	}
	h.Metadata.K0sBinaryTempFile = tmp
	return nil
}

// populateCaches downloads binaries to the local XDG cache for all upload-type providers.
// This intentionally runs in dry-run mode too: the local cache must be populated before
// Stage can upload the binary to remote hosts, and subsequent phases need it for validation.
// EnsureCached logs its own progress (info/debug), so no additional DryMsg/Wet wrapper is needed.
func (p *StageBinaries) populateCaches(ctx context.Context, hosts cluster.Hosts) error {
	seen := make(map[string]bool)
	for _, h := range hosts {
		provider, err := h.K0sBinaryProvider(p.Config.Spec.K0s.Version)
		if err != nil {
			return fmt.Errorf("%s: %w", h, err)
		}
		bc, ok := provider.(k0s.BinaryCacher)
		if !ok {
			continue
		}
		key, err := bc.BinaryCacheKey()
		if err != nil {
			return fmt.Errorf("%s: get binary cache key: %w", h, err)
		}
		if key == "" {
			return fmt.Errorf("%s: binary cache key is empty", h)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := bc.EnsureCached(ctx); err != nil {
			return fmt.Errorf("%s: cache k0s binary: %w", h, err)
		}
	}
	return nil
}

// CleanUp removes staged temp files that were not consumed (e.g. on failure).
func (p *StageBinaries) CleanUp() {
	_ = p.parallelDo(context.Background(), p.hosts, func(ctx context.Context, h *cluster.Host) error {
		if h.Metadata.K0sBinaryTempFile != "" {
			log.Debugf("%s: cleaning up k0s binary temp file", h)
		}
		if provider, err := h.K0sBinaryProvider(p.Config.Spec.K0s.Version); err == nil {
			provider.CleanUp(ctx)
		}
		h.Metadata.K0sBinaryTempFile = ""
		return nil
	})
}
