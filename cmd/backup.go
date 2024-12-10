package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/urfave/cli/v2"
)

var backupCommand = &cli.Command{
	Name:  "backup",
	Usage: "Take backup of existing clusters state",
	Flags: []cli.Flag{
		configFlag,
		dryRunFlag,
		concurrencyFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		retryIntervalFlag,
		retryTimeoutFlag,
	},
	Before: actions(initLogging, initConfig, initManager, displayLogo, displayCopyright),
	Action: func(ctx *cli.Context) error {
		backupAction := action.Backup{
			Manager: ctx.Context.Value(ctxManagerKey{}).(*phase.Manager),
		}

		if err := backupAction.Run(); err != nil {
			return fmt.Errorf("backup failed - log file saved to %s: %w", ctx.Context.Value(ctxLogFileKey{}).(string), err)
		}

		return nil
	},
}
