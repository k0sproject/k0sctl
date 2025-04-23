package cmd

import (
	"github.com/k0sproject/k0sctl/action"

	"github.com/urfave/cli/v2"
)

var configStatusCommand = &cli.Command{
	Name:  "status",
	Usage: "Show k0s dynamic config reconciliation events",
	Flags: []cli.Flag{
		configFlag,
		forceFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		timeoutFlag,
		&cli.StringFlag{
			Name:    "output",
			Usage:   "kubectl output formatting",
			Aliases: []string{"o"},
		},
	},
	Before: actions(initLogging, initConfig),
	After:  actions(cancelTimeout),
	Action: func(ctx *cli.Context) error {
		cfg, err := readConfig(ctx)
		if err != nil {
			return err
		}

		configStatusAction := action.ConfigStatus{
			Config: cfg,
			Format: ctx.String("output"),
			Writer: ctx.App.Writer,
		}

		return configStatusAction.Run(ctx.Context)
	},
}
