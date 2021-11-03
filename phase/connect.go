package phase

import (
	"strings"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
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

var retries = uint(60)

// Run the phase
func (p *Connect) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(func(h *cluster.Host) error {
		err := retry.Do(
			func() error {
				return h.Connect()
			},
			retry.OnRetry(
				func(n uint, err error) {
					log.Errorf("%s: attempt %d of %d.. failed to connect: %s", h, n+1, retries, err.Error())
				},
			),
			retry.RetryIf(
				func(err error) bool {
					return !strings.Contains(err.Error(), "no supported methods remain")
				},
			),
			retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
			retry.MaxJitter(time.Second*2),
			retry.Delay(time.Second*3),
			retry.Attempts(retries),
			retry.LastErrorOnly(true),
		)

		if err != nil {
			log.Errorf("%s: failed to connect: %s", h, err.Error())
			p.IncProp("fail-" + h.Protocol())
			return err
		}

		log.Infof("%s: connected", h)
		p.IncProp("success-" + h.Protocol())

		return nil
	})
}
