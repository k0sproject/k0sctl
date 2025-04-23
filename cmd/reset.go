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
		forceFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		timeoutFlag,
		retryIntervalFlag,
		retryTimeoutFlag,
	},
	Before: actions(initLogging, initConfig, initManager, displayCopyright),
	After:  actions(cancelTimeout),
	Action: func(ctx *cli.Context) error {
		resetAction := action.Reset{
			Manager: ctx.Context.Value(ctxManagerKey{}).(*phase.Manager),
			Stdout:  ctx.App.Writer,
		}

		if err := resetAction.Run(ctx.Context); err != nil {
			return fmt.Errorf("reset failed - log file saved to %s: %w", ctx.Context.Value(ctxLogFileKey{}).(string), err)
		}

		return nil
	},
}
