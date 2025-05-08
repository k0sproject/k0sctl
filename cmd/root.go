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
	ch := make(chan os.Signal, 2) // Buffer size 2 to catch double signals
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sigCount := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				sigCount++
				if sigCount == 1 {
					log.Warn("Aborting... Press Ctrl-C again to exit now.")
					cancel()
				} else {
					log.Error("Forced exit")
					os.Exit(130)
				}
			}
		}
	}()
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
