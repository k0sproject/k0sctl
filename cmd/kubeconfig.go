package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/urfave/cli/v2"
)

var KubeconfigPhases = []phase.Phase{
	&phase.Leader{},
	&phase.Connect{},
	&phase.DetectOS{},
	&phase.GetKubeconfig{},
	&phase.Disconnect{},
}

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
		debugFlag,
		traceFlag,
		redactFlag,
		analyticsFlag,
	},
	Before: actions(initSilentLogging, initConfig, initAnalytics),
	After: func(ctx *cli.Context) error {
		analytics.Client.Close()
		return nil
	},
	Action: func(ctx *cli.Context) error {
		manager := phase.NewManager(ctx.Context, KubeconfigPhases...)
		res := manager.Run(ctx)

		if !res.Success() {
			return res
		}

		fmt.Println(manager.Config.Metadata.Kubeconfig)

		return nil
	},
}
