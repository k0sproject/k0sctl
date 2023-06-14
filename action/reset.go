package action

import (
	"fmt"
	"os"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	log "github.com/sirupsen/logrus"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mattn/go-isatty"
)

type Reset struct {
	Config      *v1beta1.Cluster
	Concurrency int
	Force       bool
}

func (r Reset) Run() error {
	if r.Config == nil {
		return fmt.Errorf("config is nil")
	}

	if !r.Force {
		if !isatty.IsTerminal(os.Stdout.Fd()) {
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

	manager := phase.Manager{Config: r.Config, Concurrency: r.Concurrency}
	for _, h := range r.Config.Spec.Hosts {
		h.Reset = true
	}

	lockPhase := &phase.Lock{}
	manager.AddPhase(
		&phase.Connect{},
		&phase.DetectOS{},
		lockPhase,
		&phase.PrepareHosts{},
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

	if err := manager.Run(); err != nil {
		analytics.Client.Publish("reset-failure", map[string]interface{}{"clusterID": r.Config.Spec.K0s.Metadata.ClusterID})
		return err
	}

	analytics.Client.Publish("reset-success", map[string]interface{}{"duration": time.Since(start), "clusterID": r.Config.Spec.K0s.Metadata.ClusterID})

	duration := time.Since(start).Truncate(time.Second)
	text := fmt.Sprintf("==> Finished in %s", duration)
	log.Infof(phase.Colorize.Green(text).String())

	return nil
}
