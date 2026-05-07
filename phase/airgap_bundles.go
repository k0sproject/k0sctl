package phase

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/k0sproject/k0sctl/pkg/airgap"
	v1beta1 "github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// AirgapBundles uploads k0s airgap bundles to worker-capable hosts.
type AirgapBundles struct {
	GenericPhase

	plans       []airgap.Plan
	planIndexes map[*cluster.Host]int
}

// Title for the phase.
func (p *AirgapBundles) Title() string {
	return "Upload airgap bundles"
}

// Prepare plans airgap bundle placement for worker-capable Linux hosts.
func (p *AirgapBundles) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	if !airgapEnabled(config) {
		return nil
	}
	if config.Spec.K0s.Version == nil || config.Spec.K0s.Version.IsZero() {
		return fmt.Errorf("spec.k0s.version is required when airgap is enabled")
	}
	resolver, err := p.resolver()
	if err != nil {
		return err
	}
	plans, err := airgap.PlanHosts(config.Spec.Hosts, config.Spec.K0s.Version, resolver)
	if err != nil {
		return err
	}
	if config.Spec.K0s.Airgap.Source == cluster.AirgapSourceLocal {
		for i := range plans {
			plans[i].Artifact.SHA256 = config.Spec.K0s.Airgap.SHA256
		}
	}
	p.plans = plans
	p.indexPlans()
	p.warnPullPolicy()
	return nil
}

// ShouldRun is true when airgap handling is enabled and at least one host needs a bundle.
func (p *AirgapBundles) ShouldRun() bool {
	return airgapEnabled(p.Config) && len(p.plans) > 0
}

// Run uploads airgap bundles.
func (p *AirgapBundles) Run(ctx context.Context) error {
	if err := p.populateCaches(ctx); err != nil {
		return err
	}
	return p.parallelDoUpload(ctx, p.planHosts(), p.uploadForHost)
}

// DryRun reports planned airgap bundle uploads without downloading large bundles.
func (p *AirgapBundles) DryRun() error {
	for _, plan := range p.plans {
		p.DryMsgf(plan.Host, "upload airgap bundle %s (%s/%s) => %s", plan.Artifact.Name, plan.Artifact.OS, plan.Artifact.Arch, plan.Destination)
	}
	return nil
}

func airgapEnabled(config *v1beta1.Cluster) bool {
	return config != nil &&
		config.Spec != nil &&
		config.Spec.K0s != nil &&
		config.Spec.K0s.Airgap != nil &&
		config.Spec.K0s.Airgap.Enabled
}

func (p *AirgapBundles) resolver() (airgap.Resolver, error) {
	cfg := p.Config.Spec.K0s.Airgap
	switch cfg.Source {
	case cluster.AirgapSourceAuto:
		return airgap.GitHubReleaseResolver{}, nil
	case cluster.AirgapSourceURL:
		return airgap.URLResolver{Template: cfg.URL, SHA256: cfg.SHA256}, nil
	case cluster.AirgapSourceLocal:
		return airgap.GitHubReleaseResolver{}, nil
	default:
		return nil, fmt.Errorf("unsupported airgap source %q", cfg.Source)
	}
}

func (p *AirgapBundles) warnPullPolicy() {
	policy := p.Config.Spec.K0s.Config.DigString("spec", "images", "default_pull_policy")
	if policy == "Never" {
		return
	}
	log.Warn("airgap is enabled but spec.k0s.config.spec.images.default_pull_policy is not Never")
}

func (p *AirgapBundles) planHosts() cluster.Hosts {
	hosts := make(cluster.Hosts, 0, len(p.plans))
	for _, plan := range p.plans {
		hosts = append(hosts, plan.Host)
	}
	return hosts
}

func (p *AirgapBundles) indexPlans() {
	p.planIndexes = make(map[*cluster.Host]int, len(p.plans))
	for i, plan := range p.plans {
		p.planIndexes[plan.Host] = i
	}
}

func (p *AirgapBundles) populateCaches(ctx context.Context) error {
	cfg := p.Config.Spec.K0s.Airgap
	if cfg.Source == cluster.AirgapSourceLocal {
		return nil
	}
	seen := make(map[string]bool)
	for i := range p.plans {
		cachePath, err := airgap.CacheFilePath(p.Config.Spec.K0s.Version, p.plans[i].Artifact.OS, p.plans[i].Artifact.Arch, p.plans[i].Artifact.Name)
		if err != nil {
			return fmt.Errorf("%s: get airgap cache path: %w", p.plans[i].Host, err)
		}
		if seen[cachePath] {
			p.plans[i].LocalPath = cachePath
			continue
		}
		seen[cachePath] = true
		localPath, err := airgap.EnsureCached(ctx, p.Config.Spec.K0s.Version, p.plans[i].Artifact)
		if err != nil {
			return fmt.Errorf("%s: cache airgap bundle: %w", p.plans[i].Host, err)
		}
		p.plans[i].LocalPath = localPath
	}
	return nil
}

func (p *AirgapBundles) uploadForHost(ctx context.Context, h *cluster.Host) error {
	planIndex, ok := p.planIndexes[h]
	if !ok {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("upload airgap bundle canceled: %w", err)
	}
	return p.uploadBundle(p.plans[planIndex])
}

func (p *AirgapBundles) uploadBundle(plan airgap.Plan) error {
	localPath, err := p.localPath(plan)
	if err != nil {
		return fmt.Errorf("resolve local airgap bundle path: %w", err)
	}
	if err := p.verifyChecksum(localPath, plan.Artifact.SHA256); err != nil {
		return err
	}
	if err := p.ensureImagesDir(plan.Host, path.Dir(plan.Destination)); err != nil {
		return err
	}
	if !plan.Host.FileChanged(localPath, plan.Destination) {
		log.Infof("%s: airgap bundle already exists and has not changed, skipping upload", plan.Host)
		return nil
	}
	stat, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat local airgap bundle %s: %w", localPath, err)
	}
	err = p.Wet(plan.Host, fmt.Sprintf("upload airgap bundle %s => %s", localPath, plan.Destination), func() error {
		return plan.Host.Upload(localPath, plan.Destination, stat.Mode(), exec.Sudo(plan.Host), exec.LogError(true))
	})
	if err != nil {
		return err
	}
	return p.Wet(plan.Host, fmt.Sprintf("set permissions for %s to 0644", plan.Destination), func() error {
		return chmodWithMode(plan.Host, plan.Destination, fs.FileMode(0o644))
	})
}

func (p *AirgapBundles) localPath(plan airgap.Plan) (string, error) {
	cfg := p.Config.Spec.K0s.Airgap
	if cfg.Source == cluster.AirgapSourceLocal {
		localPath, err := airgap.LocalPath(cfg.Path, plan.Artifact.Name)
		if err != nil {
			return "", err
		}
		return localPath, nil
	}
	if plan.LocalPath == "" {
		return "", fmt.Errorf("airgap bundle %s was not cached", plan.Artifact.Name)
	}
	return plan.LocalPath, nil
}

func (p *AirgapBundles) verifyChecksum(localPath, expected string) error {
	if expected == "" {
		return nil
	}
	if err := airgap.VerifySHA256(localPath, expected); err != nil {
		return fmt.Errorf("verify airgap bundle checksum: %w", err)
	}
	return nil
}

func (p *AirgapBundles) ensureImagesDir(h *cluster.Host, dir string) error {
	log.Debugf("%s: ensuring airgap image directory %s", h, dir)
	if !h.Configurer.FileExist(h, dir) {
		err := p.Wet(h, fmt.Sprintf("create airgap image directory %s", dir), func() error {
			return h.SudoFsys().MkDirAll(dir, fs.FileMode(0o755))
		})
		if err != nil {
			return fmt.Errorf("create airgap image directory %s: %w", dir, err)
		}
	}
	return p.Wet(h, fmt.Sprintf("set permissions for directory %s to 0755", dir), func() error {
		return chmodWithMode(h, dir, fs.FileMode(0o755))
	})
}
