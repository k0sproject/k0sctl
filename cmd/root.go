package cmd

import (
	"github.com/urfave/cli/v2"
)

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
	},
}
