package phase

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// Lock acquires an exclusive k0sctl lock on hosts
type Lock struct {
	GenericPhase
	cfs        []func()
	instanceID string
	m          sync.Mutex
	wg         sync.WaitGroup
}

// Prepare the phase
func (p *Lock) Prepare(c *v1beta1.Cluster) error {
	p.Config = c
	hn, err := os.Hostname()
	if err != nil {
		hn = "unknown"
	}
	p.instanceID = fmt.Sprintf("%s-%d", hn, os.Getpid())
	return nil
}

// Title for the phase
func (p *Lock) Title() string {
	return "Acquire exclusive host lock"
}

// Cancel releases the lock
func (p *Lock) Cancel() {
	p.m.Lock()
	defer p.m.Unlock()
	for _, f := range p.cfs {
		f()
	}
	p.wg.Wait()
}

// CleanUp calls Cancel to release the lock
func (p *Lock) CleanUp() {
	p.Cancel()
}

// UnlockPhase returns an unlock phase for this lock phase
func (p *Lock) UnlockPhase() Phase {
	return &Unlock{Cancel: p.Cancel}
}

// Run the phase
func (p *Lock) Run(ctx context.Context) error {
	if err := p.parallelDo(ctx, p.Config.Spec.Hosts, p.startLock); err != nil {
		return err
	}
	return p.Config.Spec.Hosts.ParallelEach(ctx, p.startTicker)
}

func (p *Lock) startTicker(ctx context.Context, h *cluster.Host) error {
	p.wg.Add(1)
	lfp := h.Configurer.K0sctlLockFilePath(h)
	ticker := time.NewTicker(10 * time.Second)
	ctx, cancel := context.WithCancel(ctx)
	p.m.Lock()
	p.cfs = append(p.cfs, cancel)
	p.m.Unlock()

	go func() {
		log.Tracef("%s: started periodic update of lock file %s timestamp", h, lfp)
		for {
			select {
			case <-ticker.C:
				if err := h.Configurer.Touch(h, lfp, time.Now(), exec.Sudo(h), exec.HideCommand()); err != nil {
					log.Debugf("%s: failed to touch lock file: %s", h, err)
				}
			case <-ctx.Done():
				log.Tracef("%s: stopped lock cycle, removing file", h)
				if err := h.Configurer.DeleteFile(h, lfp); err != nil {
					log.Debugf("%s: failed to remove host lock file, k0sctl may have been previously aborted or crashed. the start of next invocation may be delayed until it expires: %s", h, err)
				}
				p.wg.Done()
				return
			}
		}
	}()

	return nil
}

func (p *Lock) startLock(ctx context.Context, h *cluster.Host) error {
	return retry.Times(ctx, 10, func(_ context.Context) error {
		return p.tryLock(h)
	})
}

func (p *Lock) tryLock(h *cluster.Host) error {
	lfp := h.Configurer.K0sctlLockFilePath(h)

	if err := h.Configurer.UpsertFile(h, lfp, p.instanceID); err != nil {
		stat, err := h.Configurer.Stat(h, lfp, exec.Sudo(h), exec.HideCommand())
		if err != nil {
			return fmt.Errorf("lock file disappeared: %w", err)
		}
		content, err := h.Configurer.ReadFile(h, lfp)
		if err != nil {
			return fmt.Errorf("failed to read lock file:  %w", err)
		}
		if content != p.instanceID {
			if time.Since(stat.ModTime()) < 30*time.Second {
				return fmt.Errorf("another instance of k0sctl is currently operating on the host, delete %s or wait 30 seconds for it to expire", lfp)
			}
			_ = h.Configurer.DeleteFile(h, lfp)
			return fmt.Errorf("removed existing expired lock file, will retry")
		}
	}

	return nil
}
