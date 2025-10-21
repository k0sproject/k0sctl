package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/a8m/envsubst"
	"github.com/adrg/xdg"
	glob "github.com/bmatcuk/doublestar/v4"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/manifest"
	"github.com/k0sproject/k0sctl/pkg/retry"
	k0sctl "github.com/k0sproject/k0sctl/version"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/exec"
	"github.com/logrusorgru/aurora"
	"github.com/shiena/ansicolor"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

type (
	ctxConfigsKey struct{}
	ctxManagerKey struct{}
	ctxLogFileKey struct{}
)

const veryLongTime = 100 * 365 * 24 * time.Hour // 100 years is infinite enough

var (
	globalCancel context.CancelFunc

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

	forceFlag = &cli.BoolFlag{
		Name:  "force",
		Usage: "Attempt a forced operation in case of certain failures",
		Action: func(c *cli.Context, force bool) error {
			phase.Force = force
			return nil
		},
	}

	configFlag = &cli.StringSliceFlag{
		Name:      "config",
		Usage:     "Path or glob to config yaml. Can be given multiple times. Use '-' to read from stdin.",
		Aliases:   []string{"c"},
		Value:     cli.NewStringSlice("k0sctl.yaml"),
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

	timeoutFlag = &cli.DurationFlag{
		Name:  "timeout",
		Usage: "Timeout for the whole operation. Set to 0 to wait forever. Can be used to allow more time for the operation to finish before giving up.",
		Value: 0,
		Action: func(ctx *cli.Context, d time.Duration) error {
			if d == 0 {
				d = veryLongTime
			}
			timeoutCtx, cancel := context.WithTimeout(ctx.Context, d)
			ctx.Context = timeoutCtx
			globalCancel = cancel
			return nil
		},
	}

	retryTimeoutFlag = &cli.DurationFlag{
		Name:   "default-timeout",
		Hidden: true,
		Usage:  "Default timeout when waiting for node state changes",
		Value:  retry.DefaultTimeout,
		Action: func(_ *cli.Context, d time.Duration) error {
			log.Warnf("default-timeout flag is deprecated and will be removed, use --timeout instead")
			retry.DefaultTimeout = d
			return nil
		},
	}

	retryIntervalFlag = &cli.DurationFlag{
		Name:   "retry-interval",
		Usage:  "Retry interval when waiting for node state changes",
		Hidden: true,
		Value:  retry.Interval,
		Action: func(_ *cli.Context, d time.Duration) error {
			log.Warnf("retry-interval flag is deprecated and will be removed")
			retry.Interval = d
			return nil
		},
	}

	Colorize = aurora.NewAurora(false)
)

func cancelTimeout(_ *cli.Context) error {
	if globalCancel != nil {
		globalCancel()
	}
	return nil
}

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
	f := ctx.StringSlice("config")
	if len(f) == 0 || f[0] == "" {
		return nil
	}

	var configs []string
	// detect globs and expand
	for _, p := range f {
		if p == "-" || p == "k0sctl.yaml" {
			configs = append(configs, p)
			continue
		}
		stat, err := os.Stat(p)
		if err == nil {
			if stat.IsDir() {
				p = path.Join(p, "**/*.{yml,yaml}")
			}
		}
		base, pattern := glob.SplitPattern(p)
		fsys := os.DirFS(base)
		matches, err := glob.Glob(fsys, pattern)
		if err != nil {
			return err
		}
		log.Debugf("glob %s expanded to %v", p, matches)
		for _, m := range matches {
			configs = append(configs, path.Join(base, m))
		}
	}

	if len(configs) == 0 {
		return fmt.Errorf("no configuration files found")
	}

	log.Debugf("%d potential configuration files found", len(configs))

	manifestReader := &manifest.Reader{}

	for _, f := range configs {
		file, err := configReader(ctx, f)
		if err != nil {
			return err
		}
		cfgFile := file
		cfgName := f
		defer func() {
			if err := cfgFile.Close(); err != nil {
				log.Warnf("failed to close config file %s: %v", cfgName, err)
			}
		}()

		content, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		subst, err := envsubst.Bytes(content)
		if err != nil {
			return err
		}
		if bytes.Equal(subst, content) {
			log.Debugf("no variable substitutions made in %s", f)
		} else {
			log.Debugf("variable substitutions made in %s, before %d after %d bytes", f, len(content), len(subst))
		}

		log.Debugf("parsing configuration from %s", f)

		if err := manifestReader.ParseBytes(subst); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}

		log.Debugf("parsed %d resource definition manifests from %s", manifestReader.Len(), f)
	}

	if manifestReader.Len() == 0 {
		return fmt.Errorf("no resource definition manifests found in configuration files")
	}

	ctx.Context = context.WithValue(ctx.Context, ctxConfigsKey{}, manifestReader)

	return nil
}

func displayCopyright(ctx *cli.Context) error {
	fmt.Fprintf(ctx.App.Writer, "k0sctl %s Copyright 2025, k0sctl authors.\n", k0sctl.Version)
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

func readConfig(ctx *cli.Context) (*v1beta1.Cluster, error) {
	mr, err := ManifestReader(ctx.Context)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest reader: %w", err)
	}
	ctlConfigs, err := mr.GetResources(v1beta1.APIVersion, "Cluster")
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster resources: %w", err)
	}
	if len(ctlConfigs) != 1 {
		return nil, fmt.Errorf("expected exactly one cluster config, got %d", len(ctlConfigs))
	}
	cfg := &v1beta1.Cluster{}
	if err := ctlConfigs[0].Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster config: %w", err)
	}
	if k0sConfigs, err := mr.GetResources("k0s.k0sproject.io/v1beta1", "ClusterConfig"); err == nil && len(k0sConfigs) > 0 {
		if cfg.Spec.K0s.Config == nil {
			cfg.Spec.K0s.Config = make(dig.Mapping)
		}
		for _, k0sConfig := range k0sConfigs {
			k0s := make(dig.Mapping)
			log.Debugf("unmarshalling %d bytes of config from %v", len(k0sConfig.Raw), k0sConfig.Filename())
			if err := k0sConfig.Unmarshal(&k0s); err != nil {
				return nil, fmt.Errorf("failed to unmarshal k0s config: %w", err)
			}
			log.Debugf("merging in k0s config from %v", k0sConfig.Filename())
			cfg.Spec.K0s.Config.Merge(k0s)
		}
	}
	otherConfigs := mr.FilterResources(func(rd *manifest.ResourceDefinition) bool {
		if strings.EqualFold(rd.APIVersion, v1beta1.APIVersion) && strings.EqualFold(rd.Kind, "cluster") {
			return false
		}
		if strings.EqualFold(rd.APIVersion, "k0s.k0sproject.io/v1beta1") && strings.EqualFold(rd.Kind, "clusterconfig") {
			return false
		}
		return true
	})
	if len(otherConfigs) > 0 {
		cfg.Metadata.Manifests = make(map[string][]byte)
		log.Debugf("found %d additional resources in the configuration", len(otherConfigs))
		for _, otherConfig := range otherConfigs {
			log.Debugf("found resource: %s (%d bytes)", otherConfig.Filename(), len(otherConfig.Raw))
			cfg.Metadata.Manifests[otherConfig.Filename()] = otherConfig.Raw
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("cluster config validation failed: %w", err)
	}
	return cfg, nil
}

func initManager(ctx *cli.Context) error {
	cfg, err := readConfig(ctx)
	if err != nil {
		return err
	}

	manager, err := phase.NewManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize phase manager: %w", err)
	}

	if ctx.IsSet("concurrency") {
		manager.Concurrency = ctx.Int("concurrency")
	} else {
		manager.Concurrency = cfg.Spec.Options.Concurrency.Limit
	}
	if ctx.IsSet("concurrent-uploads") {
		manager.ConcurrentUploads = ctx.Int("concurrent-uploads")
	} else {
		manager.ConcurrentUploads = cfg.Spec.Options.Concurrency.Uploads
	}
	manager.DryRun = ctx.Bool("dry-run")
	manager.Writer = ctx.App.Writer

	ctx.Context = context.WithValue(ctx.Context, ctxManagerKey{}, manager)

	return nil
}

// initLogging initializes the logger
func initLogging(ctx *cli.Context) error {
	log.SetLevel(log.TraceLevel)
	log.SetOutput(io.Discard)
	initScreenLogger(ctx, logLevelFromCtx(ctx, log.InfoLevel))
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
	initScreenLogger(ctx, logLevelFromCtx(ctx, log.FatalLevel))
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

func initScreenLogger(ctx *cli.Context, lvl log.Level) {
	log.AddHook(screenLoggerHook(ctx, lvl))
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
		return nil, fmt.Errorf("failed to open log %s: %s", fn, err.Error())
	}

	fmt.Fprintf(logFile, "time=\"%s\" level=info msg=\"###### New session ######\"\n", time.Now().Format(time.RFC822))

	return logFile, nil
}

func configReader(ctx *cli.Context, f string) (io.ReadCloser, error) {
	if f == "-" {
		if inF, ok := ctx.App.Reader.(*os.File); ok {
			stat, err := inF.Stat()
			if err != nil {
				return nil, fmt.Errorf("can't stat stdin: %s", err.Error())
			}
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				return inF, nil
			}
			return nil, fmt.Errorf("can't read stdin")
		}
		if inCloser, ok := ctx.App.Reader.(io.ReadCloser); ok {
			return inCloser, nil
		}
		return io.NopCloser(ctx.App.Reader), nil
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

func screenLoggerHook(ctx *cli.Context, lvl log.Level) *loghook {
	var forceColors bool
	writer := ctx.App.Writer
	if runtime.GOOS == "windows" {
		writer = ansicolor.NewAnsiColorWriter((ctx.App.Writer))
		forceColors = true
	} else {
		if outF, ok := writer.(*os.File); ok {
			if fi, _ := outF.Stat(); (fi.Mode() & os.ModeCharDevice) != 0 {
				forceColors = true
			}
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

func displayLogo(ctx *cli.Context) error {
	fmt.Fprint(ctx.App.Writer, logo)
	return nil
}

// ManifestReader returns a manifest reader from context
func ManifestReader(ctx context.Context) (*manifest.Reader, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	v := ctx.Value(ctxConfigsKey{})
	if v == nil {
		return nil, fmt.Errorf("config reader not found in context")
	}
	if r, ok := v.(*manifest.Reader); ok {
		return r, nil
	}
	return nil, fmt.Errorf("config reader in context is not of the correct type")
}
