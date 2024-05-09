package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/urfave/cli/v2"
)

var kubeconfigCommand = &cli.Command{
	Name:  "kubeconfig",
	Usage: "Output the admin kubeconfig of the cluster",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "address",
			Usage: "Set kubernetes API address (default: auto-detect)",
			Value: "",
		},
		configFlag,
		dryRunFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		retryIntervalFlag,
		retryTimeoutFlag,
	},
	Before: actions(initSilentLogging, initConfig, initManager),
	Action: func(ctx *cli.Context) error {
		kubeconfigAction := action.Kubeconfig{
			Manager:              ctx.Context.Value(ctxManagerKey{}).(*phase.Manager),
			KubeconfigAPIAddress: ctx.String("address"),
		}

		if err := kubeconfigAction.Run(); err != nil {
			return fmt.Errorf("getting kubeconfig failed - log file saved to %s: %w", ctx.Context.Value(ctxLogFileKey{}).(string), err)
		}

		_, err := fmt.Fprintf(ctx.App.Writer, "%s\n", kubeconfigAction.Manager.Config.Metadata.Kubeconfig)
		return err
	},
}
