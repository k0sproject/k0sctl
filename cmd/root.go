package cmd

import (
	"github.com/urfave/cli/v2"
)

// App is the main urfave/cli.App for k0sctl
var App = &cli.App{
	Name:  "k0sctl",
	Usage: "k0s cluster management tool",
	Flags: []cli.Flag{
		debugFlag,
		traceFlag,
	},
	Commands: []*cli.Command{
		versionCommand,
		applyCommand,
		kubeconfigCommand,
		initCommand,
		resetCommand,
	},
}
