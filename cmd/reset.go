package cmd

import (
	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"

	"github.com/urfave/cli/v2"
)

var resetCommand = &cli.Command{
	Name:  "reset",
	Usage: "Remove traces of k0s from all of the hosts",
	Flags: []cli.Flag{
		configFlag,
		concurrencyFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		retryIntervalFlag,
		retryTimeoutFlag,
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
		resetAction := action.Reset{
			Config:      ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster),
			Concurrency: ctx.Int("concurrency"),
			Force:       ctx.Bool("force"),
		}

		return resetAction.Run()
	},
}
