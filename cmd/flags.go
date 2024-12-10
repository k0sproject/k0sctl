package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/a8m/envsubst"
	"github.com/adrg/xdg"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/retry"
	k0sctl "github.com/k0sproject/k0sctl/version"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/exec"
	"github.com/logrusorgru/aurora"
	"github.com/shiena/ansicolor"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

type (
	ctxConfigKey  struct{}
	ctxManagerKey struct{}
	ctxLogFileKey struct{}
)

var (
	debugFlag = &cli.BoolFlag{
		Name:    "debug",
		Usage:   "Enable debug logging",
		Aliases: []string{"d"},
		EnvVars: []string{"DEBUG"},
	}

	dryRunFlag = &cli.BoolFlag{
		Name:    "dry-run",
		Usage:   "Do not alter cluster state, just print what would be done (EXPERIMENTAL)",
		EnvVars: []string{"DRY_RUN"},
	}

	traceFlag = &cli.BoolFlag{
		Name:    "trace",
		Usage:   "Enable trace logging",
		EnvVars: []string{"TRACE"},
		Hidden:  false,
	}

	redactFlag = &cli.BoolFlag{
		Name:  "no-redact",
		Usage: "Do not hide sensitive information in the output",
		Value: false,
	}

	configFlag = &cli.StringFlag{
		Name:      "config",
		Usage:     "Path to cluster config yaml. Use '-' to read from stdin.",
		Aliases:   []string{"c"},
		Value:     "k0sctl.yaml",
		TakesFile: true,
	}

	concurrencyFlag = &cli.IntFlag{
		Name:  "concurrency",
		Usage: "Maximum number of hosts to configure in parallel, set to 0 for unlimited",
		Value: 30,
	}

	concurrentUploadsFlag = &cli.IntFlag{
		Name:  "concurrent-uploads",
		Usage: "Maximum number of files to upload in parallel, set to 0 for unlimited",
		Value: 5,
	}

	retryTimeoutFlag = &cli.DurationFlag{
		Name:  "default-timeout",
		Usage: "Default timeout when waiting for node state changes",
		Value: retry.DefaultTimeout,
		Action: func(_ *cli.Context, d time.Duration) error {
			retry.DefaultTimeout = d
			return nil
		},
	}

	retryIntervalFlag = &cli.DurationFlag{
		Name:  "retry-interval",
		Usage: "Retry interval when waiting for node state changes",
		Value: retry.Interval,
		Action: func(_ *cli.Context, d time.Duration) error {
			retry.Interval = d
			return nil
		},
	}

	Colorize = aurora.NewAurora(false)
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

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	subst, err := envsubst.Bytes(content)
	if err != nil {
		return err
	}

	log.Debugf("Loaded configuration:\n%s", subst)

	c := &v1beta1.Cluster{}
	if err := yaml.UnmarshalStrict(subst, c); err != nil {
		return err
	}

	m, err := yaml.Marshal(c)
	if err == nil {
		log.Tracef("unmarshaled configuration:\n%s", m)
	}

	if err := c.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	ctx.Context = context.WithValue(ctx.Context, ctxConfigKey{}, c)

	return nil
}

func displayCopyright(ctx *cli.Context) error {
	fmt.Printf("k0sctl %s Copyright 2023, k0sctl authors.\n", k0sctl.Version)
	fmt.Println("By continuing to use k0sctl you agree to these terms:")
	fmt.Println("https://k0sproject.io/licenses/eula")
	return nil
}

func warnOldCache(_ *cli.Context) error {
	var olds []string
	home, err := os.UserHomeDir()
	if err == nil {
		olds = append(olds, path.Join(home, ".k0sctl", "cache"))
	}
	if runtime.GOOS == "linux" {
		olds = append(olds, "/var/cache/k0sctl")
	}
	for _, p := range olds {
		if _, err := os.Stat(p); err == nil {
			log.Warnf("An old cache directory still exists at %s, k0sctl now uses %s", p, path.Join(xdg.CacheHome, "k0sctl"))
		}
	}
	return nil
}

func initManager(ctx *cli.Context) error {
	c, ok := ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster)
	if c == nil || !ok {
		return fmt.Errorf("cluster config not available in context")
	}

	manager, err := phase.NewManager(c)
	if err != nil {
		return fmt.Errorf("failed to initialize phase manager: %w", err)
	}

	manager.Concurrency = ctx.Int("concurrency")
	manager.ConcurrentUploads = ctx.Int("concurrent-uploads")
	manager.DryRun = ctx.Bool("dry-run")

	ctx.Context = context.WithValue(ctx.Context, ctxManagerKey{}, manager)

	return nil
}

// initLogging initializes the logger
func initLogging(ctx *cli.Context) error {
	log.SetLevel(log.TraceLevel)
	log.SetOutput(io.Discard)
	initScreenLogger(logLevelFromCtx(ctx, log.InfoLevel))
	exec.DisableRedact = ctx.Bool("no-redact")
	rig.SetLogger(log.StandardLogger())
	return initFileLogger(ctx)
}

// initSilentLogging initializes the logger in silent mode
// TODO too similar to initLogging
func initSilentLogging(ctx *cli.Context) error {
	log.SetLevel(log.TraceLevel)
	log.SetOutput(io.Discard)
	exec.DisableRedact = ctx.Bool("no-redact")
	initScreenLogger(logLevelFromCtx(ctx, log.FatalLevel))
	rig.SetLogger(log.StandardLogger())
	return initFileLogger(ctx)
}

func logLevelFromCtx(ctx *cli.Context, defaultLevel log.Level) log.Level {
	if ctx.Bool("trace") {
		return log.TraceLevel
	} else if ctx.Bool("debug") {
		return log.DebugLevel
	} else {
		return defaultLevel
	}
}

func initScreenLogger(lvl log.Level) {
	log.AddHook(screenLoggerHook(lvl))
}

func initFileLogger(ctx *cli.Context) error {
	lf, err := LogFile()
	if err != nil {
		return err
	}
	log.AddHook(fileLoggerHook(lf))
	ctx.Context = context.WithValue(ctx.Context, ctxLogFileKey{}, lf.Name())
	return nil
}

const logPath = "k0sctl/k0sctl.log"

func LogFile() (*os.File, error) {
	fn, err := xdg.SearchCacheFile(logPath)
	if err != nil {
		fn, err = xdg.CacheFile(logPath)
		if err != nil {
			return nil, err
		}
	}

	logFile, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("Failed to open log %s: %s", fn, err.Error())
	}

	_, _ = fmt.Fprintf(logFile, "time=\"%s\" level=info msg=\"###### New session ######\"\n", time.Now().Format(time.RFC822))

	return logFile, nil
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

func screenLoggerHook(lvl log.Level) *loghook {
	var forceColors bool
	var writer io.Writer
	if runtime.GOOS == "windows" {
		writer = ansicolor.NewAnsiColorWriter(os.Stdout)
		forceColors = true
	} else {
		writer = os.Stdout
		if fi, _ := os.Stdout.Stat(); (fi.Mode() & os.ModeCharDevice) != 0 {
			forceColors = true
		}
	}

	if forceColors {
		Colorize = aurora.NewAurora(true)
		phase.Colorize = Colorize
	}

	l := &loghook{
		Writer:    writer,
		Formatter: &log.TextFormatter{DisableTimestamp: lvl < log.DebugLevel, ForceColors: forceColors},
	}

	l.SetLevel(lvl)

	return l
}

func fileLoggerHook(logFile io.Writer) *loghook {
	l := &loghook{
		Formatter: &log.TextFormatter{
			FullTimestamp:          true,
			TimestampFormat:        time.RFC822,
			DisableLevelTruncation: true,
		},
		Writer: logFile,
	}

	l.SetLevel(log.DebugLevel)

	return l
}

func displayLogo(_ *cli.Context) error {
	fmt.Print(logo)
	return nil
}
