package cmd

import (
	"fmt"
	"os"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	log "github.com/sirupsen/logrus"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
)

var ResetPhases = []phase.Phase{
	&phase.Connect{},
	&phase.DetectOS{},
	&phase.PrepareHosts{},
	&phase.GatherK0sFacts{},
	&phase.RunHooks{Stage: "before", Action: "reset"},
	&phase.Reset{},
	&phase.RunHooks{Stage: "after", Action: "reset"},
	&phase.Disconnect{},
}

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

		manager := phase.NewManager(ctx.Context, ResetPhases...)

		if err := analytics.Client.Publish("reset-start", map[string]interface{}{}); err != nil {
			return err
		}

		res := manager.Run(ctx)

		if !res.Success() {
			_ = analytics.Client.Publish("reset-failure", map[string]interface{}{"clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
			return res
		}

		_ = analytics.Client.Publish("reset-success", map[string]interface{}{"duration": res.Duration, "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
		text := fmt.Sprintf("==> Finished in %s", res.Duration)
		log.Infof(Colorize.Green(text).String())

		return nil
	},
}
