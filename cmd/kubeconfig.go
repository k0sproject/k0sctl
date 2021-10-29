package cmd

import (
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
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
		c := ctx.Context.Value(ctxConfigKey{}).(*config.Cluster)

		// Change so that the internal config has only single controller host as we
		// do not need to connect to all nodes
		c.Spec.Hosts = cluster.Hosts{c.Spec.K0sLeader()}
		manager := phase.Manager{Config: c}

		manager.AddPhase(
			&phase.Connect{},
			&phase.DetectOS{},
			&phase.GetKubeconfig{APIAddress: ctx.String("address")},
			&phase.Disconnect{},
		)

		return manager.Run()
	},
}
