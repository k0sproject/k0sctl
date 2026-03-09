package binprovider

import (
	"context"

	"github.com/k0sproject/k0sctl/pkg/k0s"
	log "github.com/sirupsen/logrus"
)

// existing uses a k0s binary that is already present on the host.
type existing struct {
	host Host
}

func (p *existing) NeedsUpgrade() bool {
	binary := p.host.InstalledK0sVersion()
	running := p.host.RunningK0sVersion()
	if binary == nil || running == nil {
		log.Debugf("%s: existing provider: installed=%v running=%v → needsUpgrade=false", p.host, binary, running)
		return false
	}
	result := !binary.Equal(running)
	log.Debugf("%s: existing provider: installed=%s running=%s → needsUpgrade=%v", p.host, binary, running, result)
	return result
}

func (p *existing) Stage(_ context.Context) (string, error) {
	return "", nil
}

func (p *existing) IsUpload() bool { return false }

func (p *existing) CleanUp(_ context.Context) {}

// NewExisting returns a BinaryProvider that assumes the k0s binary is already present on the host.
func NewExisting(h Host) k0s.BinaryProvider {
	return &existing{host: h}
}
