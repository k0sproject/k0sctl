package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/action"
	"github.com/urfave/cli/v2"
)

var logsCommand = &cli.Command{
	Name:  "logs",
	Usage: "Show the k0s service logs of the cluster hosts",
	Flags: []cli.Flag{
		configFlag,
		&cli.StringSliceFlag{
			Name:    "address",
			Usage:   "Only show the logs of the host with this address (can be given multiple times)",
			Aliases: []string{"a"},
		},
		&cli.StringFlag{
			Name:    "role",
			Usage:   "Only show the logs of controller or worker hosts",
			Aliases: []string{"r"},
		},
		&cli.IntFlag{
			Name:    "lines",
			Usage:   "Number of the latest lines to show per host",
			Aliases: []string{"n"},
			Value:   action.DefaultLogLines,
		},
		&cli.BoolFlag{
			Name:    "follow",
			Usage:   "Keep streaming new log lines until interrupted",
			Aliases: []string{"f"},
		},
		debugFlag,
		traceFlag,
		redactFlag,
		timeoutFlag,
	},
	Before: actions(initSilentLogging, initConfig),
	After:  actions(cancelTimeout),
	Action: func(ctx *cli.Context) error {
		cfg, err := readConfig(ctx)
		if err != nil {
			return err
		}
		if ctx.Int("lines") < 0 {
			return fmt.Errorf("--lines can't be negative")
		}

		logsAction := action.Logs{
			Config:    cfg,
			Addresses: ctx.StringSlice("address"),
			Role:      ctx.String("role"),
			Lines:     ctx.Int("lines"),
			Follow:    ctx.Bool("follow"),
			Writer:    ctx.App.Writer,
			ErrWriter: ctx.App.ErrWriter,
		}

		return logsAction.Run(ctx.Context)
	},
}
