package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/a8m/envsubst"
	"github.com/adrg/xdg"
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/integration/github"
	"github.com/k0sproject/k0sctl/integration/segment"
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

type ctxConfigKey struct{}

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

	analyticsFlag = &cli.BoolFlag{
		Name:    "disable-telemetry",
		Usage:   "Do not send anonymous telemetry",
		EnvVars: []string{"DISABLE_TELEMETRY"},
	}

	upgradeCheckFlag = &cli.BoolFlag{
		Name:    "disable-upgrade-check",
		Usage:   "Do not check for a k0sctl upgrade",
		EnvVars: []string{"DISABLE_UPGRADE_CHECK"},
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
	if !ctx.Bool("disable-telemetry") {
		fmt.Println("Anonymized telemetry of usage will be sent to the authors.")
	}
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

const segmentWriteKey string = "oU2iC4shRUBfEboaO0FDuDIUk49Ime92"

func initAnalytics(ctx *cli.Context) error {
	if ctx.Bool("disable-telemetry") {
		log.Tracef("disabling telemetry")
		return nil
	}

	client, err := segment.NewClient(segmentWriteKey)
	if err != nil {
		return err
	}
	analytics.Client = client

	return nil
}

func closeAnalytics(_ *cli.Context) error {
	analytics.Client.Close()
	return nil
}

// initLogging initializes the logger
func initLogging(ctx *cli.Context) error {
	log.SetLevel(log.TraceLevel)
	log.SetOutput(io.Discard)
	initScreenLogger(logLevelFromCtx(ctx, log.InfoLevel))
	exec.DisableRedact = ctx.Bool("no-redact")
	rig.SetLogger(log.StandardLogger())
	return initFileLogger()
}

// initSilentLogging initializes the logger in silent mode
// TODO too similar to initLogging
func initSilentLogging(ctx *cli.Context) error {
	log.SetLevel(log.TraceLevel)
	log.SetOutput(io.Discard)
	exec.DisableRedact = ctx.Bool("no-redact")
	initScreenLogger(logLevelFromCtx(ctx, log.FatalLevel))
	rig.SetLogger(log.StandardLogger())
	return initFileLogger()
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

func initFileLogger() error {
	lf, err := LogFile()
	if err != nil {
		return err
	}
	log.AddHook(fileLoggerHook(lf))
	return nil
}

const logPath = "k0sctl/k0sctl.log"

func LogFile() (io.Writer, error) {
	fn, err := xdg.SearchCacheFile(logPath)
	if err != nil {
		fn, err = xdg.CacheFile(logPath)
		if err != nil {
			return nil, err
		}
	}

	logFile, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC, 0600)
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

var upgradeChan = make(chan *github.Release)

func githubOrCachedRelease() (*github.Release, error) {
	cached, err := xdg.SearchCacheFile("k0sctl.github.latest.json")
	if err == nil {
		log.Tracef("found a cached github response in %s", cached)
		stat, err := os.Stat(cached)
		if err == nil && time.Since(stat.ModTime()) < time.Hour {
			log.Tracef("cached github release is fresh enough")
			if content, err := os.ReadFile(cached); err == nil {
				release := &github.Release{}
				if err := json.Unmarshal(content, release); err == nil {
					log.Tracef("json unmarshal ok, returning")
					return release, nil
				}
			}
		}
	}
	log.Tracef("starting online k0sctl upgrade check")
	latest, err := github.LatestRelease(k0sctl.IsPre())
	if err != nil {
		return nil, err
	}
	cached, err = xdg.CacheFile("k0sctl.github.latest.json")
	if err != nil {
		return nil, err
	}

	cf, err := os.Create(cached)
	if err != nil {
		return nil, err
	}
	log.Tracef("caching github response to %s", cached)
	enc := json.NewEncoder(cf)
	if err := enc.Encode(latest); err != nil {
		log.Tracef("failed to cache the response: %s", err)
	}
	return &latest, nil
}

func startCheckUpgrade(ctx *cli.Context) error {
	if ctx.Bool("disable-upgrade-check") || k0sctl.Environment == "development" {
		return nil
	}

	go func() {
		log.Tracef("starting k0sctl upgrade check")
		latest, err := githubOrCachedRelease()
		log.Tracef("upgrade check response received")
		if err != nil {
			log.Debugf("upgrade check failed: %s", err)
			upgradeChan <- nil
			return
		}
		if latest.IsNewer(k0sctl.Version) {
			upgradeChan <- latest
		} else {
			upgradeChan <- nil
		}
	}()

	return nil
}

func reportCheckUpgrade(ctx *cli.Context) error {
	if ctx.Bool("disable-upgrade-check") || k0sctl.Environment == "development" {
		return nil
	}

	log.Tracef("waiting for upgrade check response")
	var release *github.Release
	select {
	case release = <-upgradeChan:
		log.Tracef("upgrade check response received")
		if release == nil {
			log.Tracef("no upgrade available")
		} else {
			fmt.Println(Colorize.BrightCyan(fmt.Sprintf("A new version %s of k0sctl is available: %s", release.TagName, release.URL)))
		}
	case <-time.After(5 * time.Second):
		log.Tracef("upgrade check timed out")
	}

	return nil
}
