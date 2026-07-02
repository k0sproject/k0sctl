package action

import (
	"context"
	"fmt"
	"io"
	"time"

	log "github.com/k0sproject/k0sctl/internal/log"
	"github.com/k0sproject/k0sctl/phase"
)

type Backup struct {
	// Manager is the phase manager
	Manager *phase.Manager
	Out     io.Writer
}

func (b Backup) Run(ctx context.Context) error {
	start := time.Now()

	lockPhase := &phase.Lock{}

	b.Manager.AddPhase(
		&phase.DefaultK0sVersion{},
		&phase.Connect{},
		&phase.DetectOS{},
		lockPhase,
		&phase.PrepareHosts{},
		&phase.GatherFacts{SkipMachineIDs: true},
		&phase.GatherK0sFacts{},
		&phase.Backup{Out: b.Out},
		&phase.Unlock{Cancel: lockPhase.Cancel},
		&phase.Disconnect{},
	)

	if err := b.Manager.Run(ctx); err != nil {
		return err
	}

	duration := time.Since(start).Truncate(time.Second)
	text := fmt.Sprintf("==> Finished in %s", duration)
	log.Info(text)
	return nil
}
