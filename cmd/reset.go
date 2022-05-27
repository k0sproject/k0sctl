package cmd

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
	"github.com/urfave/cli/v2"
)

var resetCommand = &cli.Command{
	Name:  "reset",
	Usage: "Remove traces of k0s from all of the hosts",
	Flags: []cli.Flag{
		configFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		analyticsFlag,
		upgradeCheckFlag,
		&cli.BoolFlag{
			Name:    "force",
			Usage:   "Don't ask for confirmation",
			Aliases: []string{"f"},
		},
	},
	Before: actions(initLogging, startCheckUpgrade, initConfig, initAnalytics, displayCopyright),
	After:  actions(reportCheckUpgrade, closeAnalytics),
	Action: func(ctx *cli.Context) error {
		if !ctx.Bool("force") {
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

		manager := phase.Manager{Config: ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster)}

		lockPhase := &phase.Lock{}
		manager.AddPhase(
			&phase.Connect{},
			&phase.DetectOS{},
			lockPhase,
			&phase.PrepareHosts{},
			&phase.GatherK0sFacts{},
			&phase.RunHooks{Stage: "before", Action: "reset"},
			&phase.Reset{},
			&phase.RunHooks{Stage: "after", Action: "reset"},
			&phase.Unlock{Cancel: lockPhase.Cancel},
			&phase.Disconnect{},
		)

		analytics.Client.Publish("reset-start", map[string]interface{}{})

		if err := manager.Run(); err != nil {
			analytics.Client.Publish("reset-failure", map[string]interface{}{"clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
			return err
		}

		analytics.Client.Publish("reset-success", map[string]interface{}{"duration": time.Since(start), "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})

		duration := time.Since(start).Truncate(time.Second)
		text := fmt.Sprintf("==> Finished in %s", duration)
		log.Infof(Colorize.Green(text).String())

		return nil
	},
}
