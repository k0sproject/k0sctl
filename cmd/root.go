package cmd

import (
	"context"
	"io"
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

// NewK0sctl returns the urfave/cli.App for k0sctl
func NewK0sctl(in io.Reader, out, errOut io.Writer) *cli.App {
	return &cli.App{
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
		Reader:    in,
		Writer:    out,
		ErrWriter: errOut,
	}
}
