package action

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/k0sproject/k0sctl/phase"
	log "github.com/sirupsen/logrus"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mattn/go-isatty"
)

type Reset struct {
	// Manager is the phase manager
	Manager *phase.Manager
	Stdout  io.Writer
	Force   bool
}

func (r Reset) Run(ctx context.Context) error {
	if !r.Force {
		if stdoutFile, ok := r.Stdout.(*os.File); ok && !isatty.IsTerminal(stdoutFile.Fd()) {
			return fmt.Errorf("reset requires --force")
		}
		confirmed := false
		prompt := &survey.Confirm{
			Message: "Going to reset all of the hosts, which will destroy all configuration and data, Are you sure?",
		}
		_ = survey.AskOne(prompt, &confirmed)
		if !confirmed {
			return fmt.Errorf("confirmation or --force required to proceed")
		}
	}

	start := time.Now()

	for _, h := range r.Manager.Config.Spec.Hosts {
		h.Reset = true
	}

	lockPhase := &phase.Lock{}
	r.Manager.AddPhase(
		&phase.Connect{},
		&phase.DetectOS{},
		lockPhase,
		&phase.PrepareHosts{},
		&phase.GatherFacts{SkipMachineIDs: true},
		&phase.GatherK0sFacts{},
		&phase.ResetWorkers{
			NoDrain:  true,
			NoDelete: true,
		},
		&phase.ResetControllers{
			NoDrain:  true,
			NoDelete: true,
			NoLeave:  true,
		},
		&phase.ResetLeader{},
		&phase.DaemonReload{},
		&phase.Unlock{Cancel: lockPhase.Cancel},
		&phase.Disconnect{},
	)

	if err := r.Manager.Run(ctx); err != nil {
		return err
	}

	duration := time.Since(start).Truncate(time.Second)
	text := fmt.Sprintf("==> Finished in %s", duration)
	log.Info(phase.Colorize.Green(text).String())

	return nil
}
