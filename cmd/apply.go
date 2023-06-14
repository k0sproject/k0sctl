package cmd

import (
	"os"

	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"

	"github.com/urfave/cli/v2"
)

var applyCommand = &cli.Command{
	Name:  "apply",
	Usage: "Apply a k0sctl configuration",
	Flags: []cli.Flag{
		configFlag,
		concurrencyFlag,
		concurrentUploadsFlag,
		&cli.BoolFlag{
			Name:  "no-wait",
			Usage: "Do not wait for worker nodes to join",
		},
		&cli.BoolFlag{
			Name:  "no-drain",
			Usage: "Do not drain worker nodes when upgrading",
		},
		&cli.StringFlag{
			Name:      "restore-from",
			Usage:     "Path to cluster backup archive to restore the state from",
			TakesFile: true,
		},
		&cli.StringFlag{
			Name:      "kubeconfig-out",
			Usage:     "Write kubeconfig to given path after a successful apply",
			TakesFile: true,
		},
		&cli.StringFlag{
			Name:  "kubeconfig-api-address",
			Usage: "Override the API address in the kubeconfig when kubeconfig-out is set",
		},
		&cli.BoolFlag{
			Name:   "disable-downgrade-check",
			Usage:  "Skip downgrade check",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:  "force",
			Usage: "Attempt a forced installation in case of certain failures",
		},
		debugFlag,
		traceFlag,
		redactFlag,
		retryIntervalFlag,
		retryTimeoutFlag,
		analyticsFlag,
		upgradeCheckFlag,
	},
	Before: actions(initLogging, startCheckUpgrade, initConfig, displayLogo, initAnalytics, displayCopyright, warnOldCache),
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

		applier := action.Apply{
			Force:                 ctx.Bool("force"),
			NoWait:                ctx.Bool("no-wait"),
			NoDrain:               ctx.Bool("no-drain"),
			Config:                ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster),
			Concurrency:           ctx.Int("concurrency"),
			ConcurrentUploads:     ctx.Int("concurrent-uploads"),
			DisableDowngradeCheck: ctx.Bool("disable-downgrade-check"),
			KubeconfigOut:         ctx.String("kubeconfig-out"),
			KubeconfigAPIAddress:  ctx.String("kubeconfig-api-address"),
			RestoreFrom:           ctx.String("restore-from"),
			LogFile:               lf,
		}

		return applier.Run()
	},
}
