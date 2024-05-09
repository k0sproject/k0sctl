package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/phase"

	"github.com/urfave/cli/v2"
)

var applyCommand = &cli.Command{
	Name:  "apply",
	Usage: "Apply a k0sctl configuration",
	Flags: []cli.Flag{
		configFlag,
		concurrencyFlag,
		concurrentUploadsFlag,
		dryRunFlag,
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
	},
	Before: actions(initLogging, initConfig, initManager, displayLogo, displayCopyright, warnOldCache),
	Action: func(ctx *cli.Context) error {
		var kubeconfigOut io.Writer

		if kc := ctx.String("kubeconfig-out"); kc != "" {
			out, err := os.OpenFile(kc, os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				return fmt.Errorf("failed to open kubeconfig-out file: %w", err)
			}
			defer out.Close()
			kubeconfigOut = out
		}

		applyAction := action.Apply{
			Force:                 ctx.Bool("force"),
			Manager:               ctx.Context.Value(ctxManagerKey{}).(*phase.Manager),
			KubeconfigOut:         kubeconfigOut,
			KubeconfigAPIAddress:  ctx.String("kubeconfig-api-address"),
			NoWait:                ctx.Bool("no-wait"),
			NoDrain:               ctx.Bool("no-drain"),
			DisableDowngradeCheck: ctx.Bool("disable-downgrade-check"),
			RestoreFrom:           ctx.String("restore-from"),
		}

		if err := applyAction.Run(); err != nil {
			return fmt.Errorf("apply failed - log file saved to %s: %w", ctx.Context.Value(ctxLogFileKey{}).(string), err)
		}

		return nil
	},
}
