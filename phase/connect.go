package phase

import (
	"context"
	"errors"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig"
	log "github.com/sirupsen/logrus"
)

// Connect connects to each of the hosts
type Connect struct {
	GenericPhase
}

// Title for the phase
func (p *Connect) Title() string {
	return "Connect to hosts"
}

// Run the phase
func (p *Connect) Run() error {
	return p.parallelDo(p.Config.Spec.Hosts, func(h *cluster.Host) error {
		return retry.Timeout(context.TODO(), 10*time.Minute, func(_ context.Context) error {
			if err := h.Connect(); err != nil {
				if errors.Is(err, rig.ErrCantConnect) {
					return errors.Join(retry.ErrAbort, err)
				}
				return err
			}

			log.Infof("%s: connected", h)

			return nil
		})
	})
}
