package cmd

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

var validateCommand = &cli.Command{
	Name:  "validate",
	Usage: "Validate a k0sctl configuration",
	Flags: []cli.Flag{
		configFlag,
	},
	Before: actions(initLogging, initConfig, displayLogo, displayCopyright, warnOldCache),
	After:  actions(cancelTimeout),
	Action: func(ctx *cli.Context) error {
		if _, err := readConfig(ctx); err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}
		fmt.Fprintln(ctx.App.Writer, "Configuration is valid")
		return nil
	},
}
