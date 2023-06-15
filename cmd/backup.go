package cmd

import (
	"os"

	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/urfave/cli/v2"
)

var backupCommand = &cli.Command{
	Name:  "backup",
	Usage: "Take backup of existing clusters state",
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
	},
	Before: actions(initLogging, startCheckUpgrade, initConfig, displayLogo, initAnalytics, displayCopyright),
	After:  actions(reportCheckUpgrade, closeAnalytics),
	Action: func(ctx *cli.Context) error {
		logWriter, err := LogFile()
		if err != nil {
			return err
		}

		var lf *os.File

		if l, ok := logWriter.(*os.File); ok && l != nil {
			lf = l
		}

		backupAction := action.Backup{
			Config:      ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster),
			Concurrency: ctx.Int("concurrency"),
			LogFile:     lf,
		}

		return backupAction.Run()
	},
}
