package cmd

import (
	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"

	"github.com/urfave/cli/v2"
)

var configStatusCommand = &cli.Command{
	Name:  "status",
	Usage: "Show k0s dynamic config reconciliation events",
	Flags: []cli.Flag{
		configFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		&cli.StringFlag{
			Name:    "output",
			Usage:   "kubectl output formatting",
			Aliases: []string{"o"},
		},
	},
	Before: actions(initLogging, initConfig),
	Action: func(ctx *cli.Context) error {
		configStatusAction := action.ConfigStatus{
			Config: ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster),
			Format: ctx.String("output"),
			Writer: ctx.App.Writer,
		}

		return configStatusAction.Run()
	},
}
