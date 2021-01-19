package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/k0sproject/k0sctl/version"
	"github.com/k0sproject/rig"
	"github.com/shiena/ansicolor"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var (
	debugFlag = &cli.BoolFlag{
		Name:    "debug",
		Usage:   "Enable debug logging",
		Aliases: []string{"d"},
		EnvVars: []string{"DEBUG"},
	}

	traceFlag = &cli.BoolFlag{
		Name:    "trace",
		Usage:   "Enable trace logging",
		Aliases: []string{"dd"},
		EnvVars: []string{"TRACE"},
		Hidden:  true,
	}

	configFlag = &cli.StringFlag{
		Name:      "config",
		Usage:     "Path to cluster config yaml. Use '-' to read from stdin.",
		Aliases:   []string{"c"},
		Value:     "k0sctl.yaml",
		TakesFile: true,
	}
)

// actions can be used to chain action functions (for urfave/cli's Before, After, etc)
func actions(funcs ...func(*cli.Context) error) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		for _, f := range funcs {
			if err := f(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

// initConfig takes the config flag, does some magic and replaces the value with the file contents
func initConfig(ctx *cli.Context) error {
	f := ctx.String("config")
	if f == "" {
		return nil
	}

	file, err := configReader(f)
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	ctx.Set("config", string(content))

	return nil
}

func displayCopyright(_ *cli.Context) error {
	log.Infof("K0sctl %s Copyright 2021, Mirantis Inc.", version.Version)
	log.Infof("Anonymized telemetry will be sent to Mirantis.")
	log.Infof("By continuing to use k0sctl you agree to these terms.")
	return nil
}

// initLogging initializes the logger
func initLogging(ctx *cli.Context) error {
	log.SetLevel(log.TraceLevel)
	log.SetOutput(ioutil.Discard) // Send all logs to nowhere by default

	screen := screenLoggerHook()
	if ctx.Bool("debug") {
		screen.SetLevel(log.DebugLevel)
	} else if ctx.Bool("trace") {
		screen.SetLevel(log.TraceLevel)
	} else {
		screen.SetLevel(log.InfoLevel)
	}

	log.AddHook(screen)

	rig.SetLogger(log.StandardLogger())

	return nil
}

func configReader(f string) (io.ReadCloser, error) {
	if f == "-" {
		stat, err := os.Stdin.Stat()
		if err != nil {
			return nil, fmt.Errorf("can't stat stdin: %s", err.Error())
		}
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			return os.Stdin, nil
		}
		return nil, fmt.Errorf("can't read stdin")
	}

	variants := []string{f}
	// add .yml to default value lookup
	if f == "k0sctl.yaml" {
		variants = append(variants, "k0sctl.yml")
	}

	for _, fn := range variants {
		if _, err := os.Stat(fn); err != nil {
			continue
		}

		fp, err := filepath.Abs(fn)
		if err != nil {
			return nil, err
		}
		file, err := os.Open(fp)
		if err != nil {
			return nil, err
		}

		return file, nil
	}

	return nil, fmt.Errorf("failed to locate configuration")
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
