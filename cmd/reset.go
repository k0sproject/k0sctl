package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

var resetCommand = &cli.Command{
	Name:  "reset",
	Usage: "Remove traces of k0s from all of the hosts",
	Flags: []cli.Flag{
		configFlag,
		debugFlag,
		traceFlag,
		analyticsFlag,
		&cli.BoolFlag{
			Name:    "force",
			Usage:   "Don't ask for confirmation",
			Aliases: []string{"f"},
		},
	},
	Before: actions(initLogging, initConfig, initAnalytics, displayCopyright),
	After: func(ctx *cli.Context) error {
		analytics.Client.Close()
		return nil
	},
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
		content := ctx.String("config")

		c := config.Cluster{}
		if err := yaml.UnmarshalStrict([]byte(content), &c); err != nil {
			return err
		}

		if err := c.Validate(); err != nil {
			return err
		}

		manager := phase.Manager{Config: &c}

		manager.AddPhase(
			&phase.Connect{},
			&phase.DetectOS{},
			&phase.PrepareHosts{},
			&phase.GatherK0sFacts{},
			&phase.Reset{},
			&phase.Disconnect{},
		)

		if err := analytics.Client.Publish("reset-start", map[string]interface{}{}); err != nil {
			return err
		}

		if err := manager.Run(); err != nil {
			_ = analytics.Client.Publish("reset-failure", map[string]interface{}{"clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
			return err
		}

		_ = analytics.Client.Publish("reset-success", map[string]interface{}{"duration": time.Since(start), "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})

		duration := time.Since(start).Truncate(time.Second)
		text := fmt.Sprintf("==> Finished in %s", duration)
		log.Infof(aurora.Green(text).String())

		return nil
	},
}
