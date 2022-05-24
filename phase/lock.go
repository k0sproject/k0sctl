package phase

import (
	"context"
	"fmt"
	gos "os"
	"sync"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// Lock acquires an exclusive k0sctl lock on hosts
type Lock struct {
	GenericPhase
	cfs        []func()
	instanceID string
	m          sync.Mutex
}

// Prepare the phase
func (p *Lock) Prepare(c *v1beta1.Cluster) error {
	p.Config = c
	mid, _ := analytics.MachineID()
	p.instanceID = fmt.Sprintf("%s-%d", mid, gos.Getpid())
	return nil
}

// Title for the phase
func (p *Lock) Title() string {
	return "Acquire exclusive host lock"
}

func (p *Lock) Cancel() {
	p.m.Lock()
	defer p.m.Unlock()
	for _, f := range p.cfs {
		f()
	}
}

// Run the phase
func (p *Lock) Run() error {
	if err := p.Config.Spec.Hosts.ParallelEach(p.startLock); err != nil {
		return err
	}
	return p.Config.Spec.Hosts.ParallelEach(p.startTicker)
}

func (p *Lock) startTicker(h *cluster.Host) error {
	lfp := h.Configurer.K0sctlLockFilePath()
	ticker := time.NewTicker(10 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	p.m.Lock()
	p.cfs = append(p.cfs, cancel)
	p.m.Unlock()

	go func() {
		log.Debugf("%s: started periodic update of lock file %s timestamp", h, lfp)
		for {
			select {
			case <-ticker.C:
				if err := h.Configurer.Touch(h, h.Configurer.K0sctlLockFilePath(), time.Now(), exec.Sudo(h)); err != nil {
					log.Debugf("%s: failed to touch lock file: %s", h, err)
				}
			case <-ctx.Done():
				_ = h.Configurer.DeleteFile(h, lfp)
				return
			}
		}
	}()

	return nil
}

func (p *Lock) startLock(h *cluster.Host) error {
	return retry.Do(
		func() error {
			return p.tryLock(h)
		},
		retry.OnRetry(
			func(n uint, err error) {
				log.Errorf("%s: attempt %d of %d.. trying to obtain a lock on host: %s", h, n+1, retries, err.Error())
			},
		),
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.MaxJitter(time.Second*2),
		retry.Delay(time.Second*3),
		retry.Attempts(5),
		retry.LastErrorOnly(true),
	)
}

func (p *Lock) tryLock(h *cluster.Host) error {
	lfp := h.Configurer.K0sctlLockFilePath()

	if err := h.Configurer.UpsertFile(h, lfp, p.instanceID); err != nil {
		stat, err := h.Configurer.Stat(h, lfp, exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("lock file disappeared: %w", err)
		}
		content, err := h.Configurer.ReadFile(h, lfp)
		if err != nil {
			return fmt.Errorf("failed to read lock file:  %w", err)
		}
		if content != p.instanceID {
			if time.Since(stat.ModTime()) < 20*time.Second {
				return fmt.Errorf("another instance of k0sctl is currently operating on the host")
			}
			_ = h.Configurer.DeleteFile(h, lfp)
			return fmt.Errorf("removed existing expired lock file")
		}
	}

	return nil
}
