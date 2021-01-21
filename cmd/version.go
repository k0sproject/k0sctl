package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/version"
	"github.com/urfave/cli/v2"
)

var versionCommand = &cli.Command{
	Name:  "version",
	Usage: "Output k0sctl version",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:   "machine-id",
			Hidden: true,
		},
	},
	Action: func(ctx *cli.Context) error {
		fmt.Printf("version: %s\n", version.Version)
		fmt.Printf("commit: %s\n", version.GitCommit)
		if ctx.Bool("machine-id") {
			id, err := analytics.MachineID()
			if err != nil {
				id = "failed: " + err.Error()
			}
			fmt.Printf("machine-id: %s\n", id)
		}
		return nil
	},
}
