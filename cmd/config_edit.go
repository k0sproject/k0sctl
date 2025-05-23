package cmd

import (
	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"

	"github.com/urfave/cli/v2"
)

var configEditCommand = &cli.Command{
	Name:  "edit",
	Usage: "Edit k0s dynamic config in SHELL's default editor",
	Flags: []cli.Flag{
		configFlag,
		debugFlag,
		forceFlag,
		traceFlag,
		redactFlag,
		timeoutFlag,
	},
	Before: actions(initLogging, initConfig),
	After:  actions(cancelTimeout),
	Action: func(ctx *cli.Context) error {
		configEditAction := action.ConfigEdit{
			Config: ctx.Context.Value(ctxConfigsKey{}).(*v1beta1.Cluster),
			Stdout: ctx.App.Writer,
			Stderr: ctx.App.ErrWriter,
			Stdin:  ctx.App.Reader,
		}

		return configEditAction.Run(ctx.Context)
	},
}
