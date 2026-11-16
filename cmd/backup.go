package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/k0sproject/k0sctl/action"
	"github.com/k0sproject/k0sctl/phase"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var backupCommand = &cli.Command{
	Name:  "backup",
	Usage: "Take backup of existing clusters state",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output path for the backup. Default is k0s_backup_<timestamp>.tar.gz in current directory",
		},
		configFlag,
		dryRunFlag,
		concurrencyFlag,
		forceFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		timeoutFlag,
		retryIntervalFlag,
		retryTimeoutFlag,
	},
	Before: actions(initLogging, initConfig, initManager, displayLogo, displayCopyright),
	After:  actions(cancelTimeout),
	Action: func(ctx *cli.Context) error {
		var resultErr error
		var out io.Writer
		localFile := ctx.String("output")

		if localFile == "" {
			f, err := filepath.Abs(fmt.Sprintf("k0s_backup_%d.tar.gz", time.Now().Unix()))
			if err != nil {
				resultErr = fmt.Errorf("failed to generate local filename: %w", err)
				return resultErr
			}
			localFile = f
		}

		defer func() {
			if out != nil && resultErr != nil && localFile != "" {
				if err := os.Remove(localFile); err != nil {
					log.Warnf("failed to clean up incomplete backup file %s: %s", localFile, err)
				}
			}
		}()

		if out == nil {
			f, err := os.OpenFile(localFile, os.O_RDWR|os.O_CREATE|os.O_SYNC, 0o600)
			if err != nil {
				resultErr = fmt.Errorf("open local file for writing: %w", err)
				return resultErr
			}
			backupFile := f
			defer func() {
				if err := backupFile.Close(); err != nil {
					log.Warnf("failed to close backup file %s: %v", localFile, err)
				}
			}()
			out = f
		}

		backupAction := action.Backup{
			Manager: ctx.Context.Value(ctxManagerKey{}).(*phase.Manager),
			Out:     out,
		}

		if err := backupAction.Run(ctx.Context); err != nil {
			resultErr = fmt.Errorf("backup failed - log file saved to %s: %w", ctx.Context.Value(ctxLogFileKey{}).(string), err)
		}

		return resultErr
	},
}
