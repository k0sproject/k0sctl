package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func trapSignals(ctx context.Context, cancel context.CancelFunc) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ch:
		log.Warnf("received an interrupt signal, aborting operation")
		cancel()
	case <-ctx.Done():
		return
	}
}

// App is the main urfave/cli.App for k0sctl
var App = &cli.App{
	Name:  "k0sctl",
	Usage: "k0s cluster management tool",
	Flags: []cli.Flag{
		debugFlag,
		traceFlag,
		redactFlag,
	},
	Commands: []*cli.Command{
		versionCommand,
		applyCommand,
		kubeconfigCommand,
		initCommand,
		resetCommand,
		backupCommand,
		{
			Name:  "config",
			Usage: "Configuration related sub-commands",
			Subcommands: []*cli.Command{
				configEditCommand,
				configStatusCommand,
			},
		},
		completionCommand,
	},
	EnableBashCompletion: true,
	Before: func(ctx *cli.Context) error {
		if globalCancel == nil {
			cancelCtx, cancel := context.WithCancel(ctx.Context)
			ctx.Context = cancelCtx
			globalCancel = cancel
		}
		go trapSignals(ctx.Context, globalCancel)
		return nil
	},
	After: func(ctx *cli.Context) error {
		return cancelTimeout(ctx)
	},
}
