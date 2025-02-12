package phase

import (
	"context"
	"fmt"

	"github.com/k0sproject/version"

	log "github.com/sirupsen/logrus"
)

type DefaultK0sVersion struct {
	GenericPhase
}

func (p *DefaultK0sVersion) ShouldRun() bool {
	return p.Config.Spec.K0s.Version == nil || p.Config.Spec.K0s.Version.IsZero()
}

func (p *DefaultK0sVersion) Title() string {
	return "Set k0s version"
}

func (p *DefaultK0sVersion) Run(_ context.Context) error {
	isStable := p.Config.Spec.K0s.VersionChannel == "" || p.Config.Spec.K0s.VersionChannel == "stable"

	var msg string
	if isStable {
		msg = "latest stable k0s version"
	} else {
		msg = "latest k0s version including pre-releases"
	}

	log.Info("Looking up ", msg)
	latest, err := version.LatestByPrerelease(!isStable)
	if err != nil {
		return fmt.Errorf("failed to look up k0s version online - try setting spec.k0s.version manually: %w", err)
	}
	log.Infof("Using k0s version %s", latest)
	p.Config.Spec.K0s.Version = latest
	p.Config.Spec.K0s.Metadata.VersionDefaulted = true

	return nil
}
