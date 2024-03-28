package action

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
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

func (r Reset) Run() error {
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
		&phase.GatherFacts{},
		&phase.GatherK0sFacts{},
		&phase.RunHooks{Stage: "before", Action: "reset"},
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
		&phase.RunHooks{Stage: "after", Action: "reset"},
		&phase.Unlock{Cancel: lockPhase.Cancel},
		&phase.Disconnect{},
	)

	analytics.Client.Publish("reset-start", map[string]interface{}{})

	if err := r.Manager.Run(); err != nil {
		analytics.Client.Publish("reset-failure", map[string]interface{}{"clusterID": r.Manager.Config.Spec.K0s.Metadata.ClusterID})
		return err
	}

	analytics.Client.Publish("reset-success", map[string]interface{}{"duration": time.Since(start), "clusterID": r.Manager.Config.Spec.K0s.Metadata.ClusterID})

	duration := time.Since(start).Truncate(time.Second)
	text := fmt.Sprintf("==> Finished in %s", duration)
	log.Infof(phase.Colorize.Green(text).String())

	return nil
}
