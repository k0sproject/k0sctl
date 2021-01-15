package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/k0sproject/rig"
	"github.com/shiena/ansicolor"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var App = &cli.App{
	Name:  "k0sctl",
	Usage: "k0s cluster management tool",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Usage:   "Enable debug logging",
			Aliases: []string{"d"},
			EnvVars: []string{"DEBUG"},
		},
		&cli.BoolFlag{
			Name:    "trace",
			Usage:   "Enable trace logging",
			Aliases: []string{"dd"},
			EnvVars: []string{"TRACE"},
			Hidden:  true,
		},
	},
	Before: func(ctx *cli.Context) error {
		if ctx.Bool("debug") {
			initLogger(log.DebugLevel)
		} else if ctx.Bool("trace") {
			initLogger(log.TraceLevel)
		} else {
			initLogger(log.InfoLevel)
		}

		return nil
	},
	Commands: []*cli.Command{
		versionCommand,
		applyCommand,
	},
}

type loghook struct {
	Writer    io.Writer
	Formatter log.Formatter

	levels []log.Level
}

func (h *loghook) SetLevel(level log.Level) {
	h.levels = []log.Level{}
	for _, l := range log.AllLevels {
		if level >= l {
			h.levels = append(h.levels, l)
		}
	}
}

func (h *loghook) Levels() []log.Level {
	return h.levels
}

func (h *loghook) Fire(entry *log.Entry) error {
	line, err := h.Formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to format log entry: %v", err)
		return err
	}
	_, err = h.Writer.Write(line)
	return err
}

func initLogger(level log.Level) {
	log.SetLevel(log.TraceLevel)
	log.SetOutput(ioutil.Discard) // Send all logs to nowhere by default

	screen := screenLoggerHook()
	screen.SetLevel(level)
	log.AddHook(screen)

	rig.SetLogger(log.StandardLogger())
}

func screenLoggerHook() *loghook {
	l := &loghook{Formatter: &log.TextFormatter{DisableTimestamp: true, ForceColors: true}}

	if runtime.GOOS == "windows" {
		l.Writer = ansicolor.NewAnsiColorWriter(os.Stdout)
	} else {
		l.Writer = os.Stdout
	}

	return l
}
