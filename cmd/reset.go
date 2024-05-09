package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/phase"

	"github.com/urfave/cli/v2"
)

var resetCommand = &cli.Command{
	Name:  "reset",
	Usage: "Remove traces of k0s from all of the hosts",
	Flags: []cli.Flag{
		configFlag,
		concurrencyFlag,
		dryRunFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		retryIntervalFlag,
		retryTimeoutFlag,
		&cli.BoolFlag{
			Name:    "force",
			Usage:   "Don't ask for confirmation",
			Aliases: []string{"f"},
		},
	},
	Before: actions(initLogging, initConfig, initManager, displayCopyright),
	Action: func(ctx *cli.Context) error {
		resetAction := action.Reset{
			Manager: ctx.Context.Value(ctxManagerKey{}).(*phase.Manager),
			Force:   ctx.Bool("force"),
			Stdout:  ctx.App.Writer,
		}

		if err := resetAction.Run(); err != nil {
			return fmt.Errorf("reset failed - log file saved to %s: %w", ctx.Context.Value(ctxLogFileKey{}).(string), err)
		}

		return nil
	},
}
