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
	log "github.com/sirupsen/logrus"
)

// authRejectionLimit is how many attempts in a row may have their credentials
// rejected before connecting to a host gives up. At retry.Interval that is about
// a minute: long enough for a host that is still being provisioned to receive
// its authorized keys, without spending the whole connect timeout on
// credentials that are simply wrong.
const authRejectionLimit = 12

// Connect connects to each of the hosts
type Connect struct {
	GenericPhase
}

// Title for the phase
func (p *Connect) Title() string {
	return "Connect to hosts"
}

// Run the phase
func (p *Connect) Run(ctx context.Context) error {
	return p.parallelDo(ctx, p.Config.Spec.Hosts, func(ctx context.Context, h *cluster.Host) error {
		var rejections int
		return retry.Timeout(ctx, 10*time.Minute, func(ctx context.Context) error {
			err := h.Connect(ctx)
			if err == nil {
				log.Infof("%s: connected", h)
				return nil
			}
			if errors.Is(err, rig.ErrNonRetryable) || strings.Contains(err.Error(), "host key mismatch") {
				return errors.Join(retry.ErrAbort, err)
			}
			if !errors.Is(err, rig.ErrAuthFailed) {
				rejections = 0
				return err
			}
			rejections++
			if rejections >= authRejectionLimit {
				return errors.Join(retry.ErrAbort, fmt.Errorf("credentials rejected %d times in a row: %w", rejections, err))
			}
			return err
		})
	})
}
