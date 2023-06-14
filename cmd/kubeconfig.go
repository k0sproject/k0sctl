package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
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
		concurrencyFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		retryIntervalFlag,
		retryTimeoutFlag,
		analyticsFlag,
	},
	Before: actions(initSilentLogging, initConfig, initAnalytics),
	After: func(_ *cli.Context) error {
		analytics.Client.Close()
		return nil
	},
	Action: func(ctx *cli.Context) error {
		var cfg *v1beta1.Cluster
		if c, ok := ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster); ok {
			cfg = c
		} else {
			return fmt.Errorf("config is nil")
		}

		kubeconfigAction := action.Kubeconfig{
			Config:               cfg,
			Concurrency:          ctx.Int("concurrency"),
			KubeconfigAPIAddress: ctx.String("address"),
		}

		if err := kubeconfigAction.Run(); err != nil {
			return err
		}

		_, err := fmt.Fprintf(ctx.App.Writer, "%s\n", cfg.Metadata.Kubeconfig)
		return err
	},
}
