package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/urfave/cli/v2"
)

var kubeconfigCommand = &cli.Command{
	Name:  "kubeconfig",
	Usage: "Output the admin kubeconfig of the cluster",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "address",
			Value:       "",
			DefaultText: "auto-detect",
		},
		&cli.StringFlag{
			Name:        "user",
			Usage:       "Set kubernetes cluster username",
			Aliases:     []string{"u"},
			DefaultText: "admin",
		},
		&cli.StringFlag{
			Name:        "cluster",
			Usage:       "Set kubernetes cluster name",
			Aliases:     []string{"n"},
			DefaultText: "k0s-cluster",
		},
		configFlag,
		dryRunFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		retryIntervalFlag,
		retryTimeoutFlag,
		analyticsFlag,
	},
	Before: actions(initSilentLogging, initConfig, initManager, initAnalytics),
	After: func(_ *cli.Context) error {
		analytics.Client.Close()
		return nil
	},
	Action: func(ctx *cli.Context) error {
		kubeconfigAction := action.Kubeconfig{
			Manager:              ctx.Context.Value(ctxManagerKey{}).(*phase.Manager),
			KubeconfigAPIAddress: ctx.String("address"),
			KubeconfigUser:       ctx.String("user"),
			KubeconfigCluster:    ctx.String("cluster"),
		}

		if err := kubeconfigAction.Run(); err != nil {
			return fmt.Errorf("getting kubeconfig failed - log file saved to %s: %w", ctx.Context.Value(ctxLogFileKey{}).(string), err)
		}

		_, err := fmt.Fprintf(ctx.App.Writer, "%s\n", kubeconfigAction.Manager.Config.Metadata.Kubeconfig)
		return err
	},
}
