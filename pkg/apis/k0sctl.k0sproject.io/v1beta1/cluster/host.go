package cluster

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	gos "os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
	"github.com/jellydator/validation/is"
	"github.com/k0sproject/k0sctl/configurer"
	k0s "github.com/k0sproject/k0sctl/pkg/k0s"
	"github.com/k0sproject/k0sctl/pkg/k0s/binprovider"
	rig "github.com/k0sproject/rig/v2"
	rigos "github.com/k0sproject/rig/v2/os"
	"github.com/k0sproject/version"
	sloglogrus "github.com/samber/slog-logrus/v2"
	log "github.com/sirupsen/logrus"
)

var K0sForceFlagSince = version.MustParse("v1.27.4+k0s.0")

var _ binprovider.Host = (*Host)(nil)

// rigLogger bridges rig v2's slog-based logging into k0sctl's logrus output.
// It wraps the logrus standard logger (the same singleton configured in cmd's
// logging setup), so any level and hook changes applied there are reflected
// automatically. rig v2 has no global logger setter; the logger is injected
// per client via rig.WithLogger at Connect time (see Host.Connect).
var rigLogger = slog.New(sloglogrus.Option{
	Level:  slog.LevelDebug,
	Logger: log.StandardLogger(),
}.NewLogrusHandler())

// Connect establishes the connection to the host, injecting k0sctl's logger so
// that rig's internal logging is routed into k0sctl's logrus output.
func (h *Host) Connect(ctx context.Context) error {
	if h.Client == nil {
		client, err := rig.NewClient(
			rig.WithConnectionFactory(&h.CompositeConfig),
			rig.WithLogger(rigLogger),
		)
		if err != nil {
			return fmt.Errorf("create rig client: %w", err)
		}
		h.Client = client
	}
	return h.Client.Connect(ctx)
}

// String returns a human-readable description of the host, safe before Connect.
func (h *Host) String() string {
	if h.Client != nil {
		return h.Client.String()
	}
	return h.CompositeConfig.String()
}

// Host contains all the needed details to work with hosts
type Host struct {
	rig.CompositeConfig `yaml:",inline"`
	*rig.Client         `yaml:"-"`

	Role                   string            `yaml:"role"`
	Reset                  bool              `yaml:"reset,omitempty"`
	PrivateInterface       string            `yaml:"privateInterface,omitempty"`
	PrivateAddress         string            `yaml:"privateAddress,omitempty"`
	DataDir                string            `yaml:"dataDir,omitempty"`
	KubeletRootDir         string            `yaml:"kubeletRootDir,omitempty"`
	Environment            map[string]string `yaml:"environment,flow,omitempty"`
	UploadBinary           bool              `yaml:"uploadBinary,omitempty"`
	UseExistingK0s         bool              `yaml:"useExistingK0s,omitempty"`
	K0sBinaryPath          string            `yaml:"k0sBinaryPath,omitempty"`
	K0sInstallPath         string            `yaml:"k0sInstallPath,omitempty"`
	K0sDownloadURLOverride string            `yaml:"k0sDownloadURL,omitempty"`
	InstallFlags           Flags             `yaml:"installFlags,omitempty"`
	Files                  []*UploadFile     `yaml:"files,omitempty"`
	OSIDOverride           string            `yaml:"os,omitempty"`
	HostnameOverride       string            `yaml:"hostname,omitempty"`
	NoTaints               bool              `yaml:"noTaints,omitempty"`
	Hooks                  Hooks             `yaml:"hooks,omitempty"`

	Metadata   HostMetadata          `yaml:"-"`
	Configurer configurer.Configurer `yaml:"-"`
	OSRelease  *rigos.Release        `yaml:"-"`

	binaryProvider              k0s.BinaryProvider // user-set override via SetK0sBinaryProvider
	cachedDefaultProvider       k0s.BinaryProvider // auto-built default; rebuilt when target changes
	cachedDefaultProviderTarget *version.Version   // target used to build cachedDefaultProvider
}

// SetK0sBinaryProvider overrides the binary acquisition strategy for this host.
// When set, it takes precedence over the strategy inferred from the host's
// configuration fields (uploadBinary, useExistingK0s, k0sDownloadURL, etc.).
// This is the primary extension point for library users — for example, to pull
// k0s from a private OCI registry rather than from GitHub.
func (h *Host) SetK0sBinaryProvider(p k0s.BinaryProvider) {
	h.binaryProvider = p
}

// InstalledK0sVersion returns the cached k0s binary version gathered during facts collection.
func (h *Host) InstalledK0sVersion() *version.Version {
	return h.Metadata.K0sBinaryVersion
}

// RunningK0sVersion returns the cached k0s service version gathered during facts collection.
func (h *Host) RunningK0sVersion() *version.Version {
	return h.Metadata.K0sRunningVersion
}

// K0sBinaryProvider returns the effective BinaryProvider for the host.
// If one has been set via SetK0sBinaryProvider it is returned; otherwise the
// provider is inferred from the host's configuration fields and constructed
// with target as the desired k0s version. The default provider is cached per
// target version and rebuilt automatically if the target changes (e.g. after
// ValidateFacts resets a defaulted version to the running version).
// target must be non-nil when using a default provider; passing nil is only
// safe when a custom provider has been set via SetK0sBinaryProvider.
func (h *Host) K0sBinaryProvider(target *version.Version) (k0s.BinaryProvider, error) {
	if h.binaryProvider != nil {
		return h.binaryProvider, nil
	}
	if target == nil {
		return nil, fmt.Errorf("K0sBinaryProvider: target version is nil and no custom provider has been set")
	}
	if h.cachedDefaultProvider == nil || !target.Equal(h.cachedDefaultProviderTarget) {
		if !h.UseExistingK0s && h.Configurer == nil && h.K0sInstallPath == "" {
			return nil, fmt.Errorf("%s: cannot determine k0s install location: OS not yet detected (Configurer is nil) and k0sInstallPath is not set", h)
		}
		h.cachedDefaultProvider = h.defaultBinaryProvider(target)
		h.cachedDefaultProviderTarget = target
	}
	return h.cachedDefaultProvider, nil
}

func (h *Host) requireConfigurer() (configurer.Configurer, error) {
	if h.Configurer == nil {
		return nil, fmt.Errorf("%s: host configurer is not resolved", h)
	}
	return h.Configurer, nil
}

// Dir returns the directory name for the given path using the remote filesystem.
func (h *Host) Dir(path string) (string, error) {
	return h.FS().Dir(path), nil
}

// OSKind returns the host OS kind via the resolved configurer.
func (h *Host) OSKind() (string, error) {
	cfg, err := h.requireConfigurer()
	if err != nil {
		return "", err
	}
	return cfg.OSKind(), nil
}

// DownloadURL downloads a file on the host using the resolved configurer.
func (h *Host) DownloadURL(url, dest string) error {
	cfg, err := h.requireConfigurer()
	if err != nil {
		return err
	}
	return cfg.DownloadURL(h, url, dest)
}

// Touch updates file modification timestamps, creating the file if needed.
func (h *Host) Touch(path string, modTime time.Time) error {
	return h.Sudo().FS().Touch(path, modTime)
}

// DeleteFile removes a file from the host.
func (h *Host) DeleteFile(path string) error {
	return h.Sudo().FS().Remove(path)
}

func (h *Host) SetDefaults() {
	if h.OSIDOverride != "" {
		h.OSRelease = &rigos.Release{ID: h.OSIDOverride}
	}

	_ = defaults.Set(&h.CompositeConfig)

	if h.InstallFlags.Get("--single") != "" && h.InstallFlags.GetValue("--single") != "false" && h.Role != "single" {
		log.Debugf("%s: changed role from '%s' to 'single' because of --single installFlag", h, h.Role)
		h.Role = "single"
	}
	if h.InstallFlags.Get("--enable-worker") != "" && h.InstallFlags.GetValue("--enable-worker") != "false" && h.Role != "controller+worker" {
		log.Debugf("%s: changed role from '%s' to 'controller+worker' because of --enable-worker installFlag", h, h.Role)
		h.Role = "controller+worker"
	}

	if h.InstallFlags.Get("--no-taints") != "" && h.InstallFlags.GetValue("--no-taints") != "false" {
		h.NoTaints = true
	}

	if dd := h.InstallFlags.GetValue("--data-dir"); dd != "" {
		if h.DataDir != "" {
			log.Debugf("%s: changed dataDir from '%s' to '%s' because of --data-dir installFlag", h, h.DataDir, dd)
		}
		h.InstallFlags.Delete("--data-dir")
		h.DataDir = dd
	}

	if krd := h.InstallFlags.GetValue("--kubelet-root-dir"); krd != "" {
		if h.KubeletRootDir != "" {
			log.Debugf("%s: changed kubeletRootDir from '%s' to '%s' because of --kubelet-root-dir installFlag", h, h.DataDir, krd)
		}
		h.InstallFlags.Delete("--kubelet-root-dir")
		h.KubeletRootDir = krd
	}
}

func validateBalancedQuotes(val any) error {
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("invalid type")
	}

	quoteCount := make(map[rune]int)

	for i, ch := range s {
		if i > 0 && s[i-1] == '\\' {
			continue
		}

		if ch == '\'' || ch == '"' {
			quoteCount[ch]++
		}
	}

	for _, count := range quoteCount {
		if count%2 != 0 {
			return fmt.Errorf("unbalanced quotes in %s", s)
		}
	}

	return nil
}

func (h *Host) Validate() error {
	// Validate connection config only when a protocol is configured; hosts without
	// connection config are valid for unit tests and partial configurations.
	if h.SSH != nil || h.WinRM != nil || bool(h.Localhost) || h.OpenSSH != nil {
		if err := h.CompositeConfig.Validate(); err != nil {
			return err
		}
	}

	if err := validation.ValidateStruct(h,
		validation.Field(&h.Role, validation.In("controller", "worker", "controller+worker", "single").Error("unknown role "+h.Role)),
		validation.Field(&h.PrivateAddress, is.IP),
		validation.Field(&h.Files),
		validation.Field(&h.NoTaints, validation.When(h.Role != "controller+worker", validation.NotIn(true).Error("noTaints can only be true for controller+worker role"))),
		validation.Field(&h.InstallFlags, validation.Each(validation.By(validateBalancedQuotes))),
	); err != nil {
		return err
	}

	if h.UseExistingK0s {
		errs := validation.Errors{}
		if h.UploadBinary {
			errs["uploadBinary"] = fmt.Errorf("uploadBinary cannot be true when useExistingK0s is true")
		}
		if h.K0sBinaryPath != "" {
			errs["k0sBinaryPath"] = fmt.Errorf("k0sBinaryPath cannot be set when useExistingK0s is true")
		}
		if h.K0sDownloadURLOverride != "" {
			errs["k0sDownloadURL"] = fmt.Errorf("k0sDownloadURL cannot be set when useExistingK0s is true")
		}
		if len(errs) > 0 {
			return errs
		}
	}

	return nil
}

// ResolveUploadFiles resolves host file sources relative to baseDir.
func (h *Host) ResolveUploadFiles(baseDir string) error {
	for _, f := range h.Files {
		if err := f.ResolveRelativeTo(baseDir); err != nil {
			return err
		}
	}
	return nil
}

// HostMetadata resolved metadata for host
type HostMetadata struct {
	K0sBinaryVersion  *version.Version
	K0sBinaryTempFile string
	K0sRunningVersion *version.Version
	K0sInstalled      bool
	K0sExistingConfig string
	K0sNewConfig      string
	K0sTokenData      TokenData
	K0sStatusArgs     Flags
	Arch              string
	IsK0sLeader       bool
	Hostname          string
	Ready             bool
	NeedsUpgrade      bool
	MachineID         string
	DryRunFakeLeader  bool
}

// Resolve prepares host-scoped data after unmarshalling by resolving upload files
// and normalizing relative paths (like K0sBinaryPath) against baseDir.
func (h *Host) Resolve(baseDir string) error {
	if h.K0sBinaryPath != "" && !filepath.IsAbs(h.K0sBinaryPath) && baseDir != "" {
		h.K0sBinaryPath = filepath.Join(baseDir, h.K0sBinaryPath)
	}
	return h.ResolveUploadFiles(baseDir)
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml.
// The alias type prevents recursion; the client is reset so Connect re-creates
// it with any updated connection config.
func (h *Host) UnmarshalYAML(unmarshal func(any) error) error {
	type host Host
	yh := (*host)(h)

	yh.Environment = make(map[string]string)

	if err := unmarshal(yh); err != nil {
		return err
	}

	if h.Client != nil {
		h.Disconnect()
		h.Client = nil
	}

	return defaults.Set(h)
}

// Address returns an address for the host, falling back to the configured
// connection address when the client is not yet connected.
func (h *Host) Address() string {
	if h.Client != nil {
		if addr := h.Client.Address(); addr != "" {
			return addr
		}
	}
	if h.SSH != nil && h.SSH.Address != "" {
		return h.SSH.Address
	}
	if h.WinRM != nil && h.WinRM.Address != "" {
		return h.WinRM.Address
	}
	if h.OpenSSH != nil && h.OpenSSH.Address != "" {
		return h.OpenSSH.Address
	}
	return "127.0.0.1"
}

// Protocol returns host communication protocol
func (h *Host) Protocol() string {
	if h.SSH != nil {
		return "ssh"
	}

	if h.WinRM != nil {
		return "winrm"
	}

	if bool(h.Localhost) {
		return "local"
	}

	return "nil"
}

// IsWindows returns true when the detected OS is Windows
func (h *Host) IsWindows() bool {
	if h.Client != nil {
		return h.Client.IsWindows()
	}
	return h.WinRM != nil
}

// ResolveConfigurer assigns a rig-style configurer to the Host (see configurer/)
func (h *Host) ResolveConfigurer() error {
	if h.OSIDOverride != "" {
		h.OSRelease = &rigos.Release{ID: h.OSIDOverride}
	} else if h.OSRelease == nil {
		release, err := h.OS()
		if err != nil {
			return fmt.Errorf("OS detection failed: %w", err)
		}
		h.OSRelease = release
	}

	bf, ok := configurer.ResolveOSModule(h.OSRelease)
	if !ok {
		return fmt.Errorf("unsupported OS: %s", h.OSRelease.ID)
	}

	c, ok := bf().(configurer.Configurer)
	if !ok {
		return fmt.Errorf("unsupported OS: %s", h.OSRelease.ID)
	}

	h.Configurer = c
	if h.K0sInstallPath != "" {
		h.Configurer.SetPath("K0sBinaryPath", h.K0sInstallPath)
	}

	return nil
}

// K0sInstallLocation returns the k0s binary path from the K0sInstallPath field or configurer.K0sBinaryPath()
func (h *Host) K0sInstallLocation() string {
	if h.K0sInstallPath != "" {
		return h.K0sInstallPath
	}

	return h.Configurer.K0sBinaryPath()
}

// LocalK0sBinaryPath returns the local path to the developer-supplied binary, when set.
func (h *Host) LocalK0sBinaryPath() string {
	return h.K0sBinaryPath
}

// DownloadURLTemplate returns the host's k0s download override template, if any.
func (h *Host) DownloadURLTemplate() string {
	return h.K0sDownloadURLOverride
}

// K0sJoinTokenPath returns the token file path from install flags or configurer
func (h *Host) K0sJoinTokenPath() string {
	if path := h.InstallFlags.GetValue("--token-file"); path != "" {
		return path
	}

	return h.Configurer.K0sJoinTokenPath()
}

// K0sConfigPath returns the config file path from install flags or configurer
func (h *Host) K0sConfigPath() string {
	if path := h.InstallFlags.GetValue("--config"); path != "" {
		return path
	}

	if path := h.InstallFlags.GetValue("-c"); path != "" {
		return path
	}

	return h.Configurer.K0sConfigPath()
}

// Arch returns the host architecture, caching the result in metadata
func (h *Host) Arch() (string, error) {
	if h.Metadata.Arch != "" {
		return h.Metadata.Arch, nil
	}
	if h.Configurer == nil {
		return "", fmt.Errorf("host configurer is not resolved")
	}
	arch, err := h.Configurer.Arch(h)
	if err != nil {
		return "", fmt.Errorf("failed to detect host architecture: %w", err)
	}
	h.Metadata.Arch = arch
	return arch, nil
}

func (h *Host) K0sRole() string {
	switch h.Role {
	case "controller+worker", "single":
		return "controller"
	default:
		return h.Role
	}
}

func (h *Host) K0sInstallFlags() (Flags, error) {
	flags := Flags(h.InstallFlags)

	flags.AddOrReplace(fmt.Sprintf("--data-dir=%s", quote(h.Configurer, h.Configurer.HostPath(h.K0sDataDir()))))

	if h.KubeletRootDir != "" {
		flags.AddOrReplace(fmt.Sprintf(
			"--kubelet-root-dir=%s",
			quote(h.Configurer, h.Configurer.HostPath(h.KubeletRootDir)),
		))
	}

	switch h.Role {
	case "controller+worker":
		flags.AddUnlessExist("--enable-worker=true")
		if h.NoTaints {
			flags.AddUnlessExist("--no-taints=true")
		}
	case "single":
		flags.AddUnlessExist("--single=true")
	}

	if !h.Metadata.IsK0sLeader {
		flags.AddUnlessExist(fmt.Sprintf(`--token-file=%s`, quote(h.Configurer, h.Configurer.HostPath(h.K0sJoinTokenPath()))))
	}

	if h.IsController() {
		flags.AddUnlessExist(fmt.Sprintf(`--config=%s`, quote(h.Configurer, h.Configurer.HostPath(h.K0sConfigPath()))))
	}

	if strings.HasSuffix(h.Role, "worker") {
		var extra Flags
		if old := flags.GetValue("--kubelet-extra-args"); old != "" {
			ex, err := NewFlags(old)
			if err != nil {
				return flags, fmt.Errorf("failed to split kubelet-extra-args: %w", err)
			}
			extra = ex
		}
		// set worker's private address to --node-ip in --extra-kubelet-args if cloud ins't enabled
		enableCloudProvider, err := h.InstallFlags.GetBoolean("--enable-cloud-provider")
		if err != nil {
			return flags, fmt.Errorf("--enable-cloud-provider flag is set to invalid value: %s. (%v)", h.InstallFlags.GetValue("--enable-cloud-provider"), err)
		}
		if !enableCloudProvider && h.PrivateAddress != "" {
			extra.AddUnlessExist("--node-ip=" + h.PrivateAddress)
		}

		if h.HostnameOverride != "" {
			extra.AddOrReplace("--hostname-override=" + h.HostnameOverride)
		}
		if extra != nil {
			flags.AddOrReplace(fmt.Sprintf("--kubelet-extra-args=%s", quote(h.Configurer, extra.Join(h.Configurer))))
		}
	}

	if flags.Include("--force") && h.Metadata.K0sBinaryVersion != nil && h.Metadata.K0sBinaryVersion.LessThan(K0sForceFlagSince) {
		log.Warnf("%s: k0s version %s does not support the --force flag, ignoring it", h, h.Metadata.K0sBinaryVersion)
		flags.Delete("--force")
	}

	return flags, nil
}

// K0sInstallCommand returns a full command that will install k0s service with necessary flags
func (h *Host) K0sInstallCommand() (string, error) {
	flags, err := h.K0sInstallFlags()
	if err != nil {
		return "", err
	}

	return h.Configurer.K0sCmdf("install %s %s", h.K0sRole(), flags.Join(h.Configurer)), nil
}

// K0sResetCommand returns a full command that will reset k0s
func (h *Host) K0sResetCommand() string {
	var flags Flags
	flags.Add(fmt.Sprintf("--data-dir=%s", quote(h.Configurer, h.Configurer.HostPath(h.K0sDataDir()))))
	if h.KubeletRootDir != "" {
		flags.Add(fmt.Sprintf(
			"--kubelet-root-dir=%s",
			quote(h.Configurer, h.Configurer.HostPath(h.KubeletRootDir)),
		))
	}

	return h.Configurer.K0sCmdf("reset %s", flags.Join(h.Configurer))
}

// K0sBackupCommand returns a full command to be used as run k0s backup
func (h *Host) K0sBackupCommand(targetDir string) string {
	return h.Configurer.K0sCmdf("backup --save-path %s --data-dir %s", quote(h.Configurer, h.Configurer.HostPath(targetDir)), h.Configurer.HostPath(h.K0sDataDir()))
}

// K0sRestoreCommand returns a full command to restore cluster state from a backup
func (h *Host) K0sRestoreCommand(backupfile string) string {
	return h.Configurer.K0sCmdf("restore --data-dir=%s %s", h.Configurer.HostPath(h.K0sDataDir()), quote(h.Configurer, h.Configurer.HostPath(backupfile)))
}

// IsController returns true for controller and controller+worker roles
func (h *Host) IsController() bool {
	return h.Role == "controller" || h.Role == "controller+worker" || h.Role == "single"
}

// K0sServiceName returns correct service name
func (h *Host) K0sServiceName() string {
	switch h.Role {
	case "controller", "controller+worker", "single":
		return "k0scontroller"
	default:
		return "k0sworker"
	}
}

func (h *Host) k0sBinaryPathDir() string {
	return path.Dir(h.K0sInstallLocation())
}

// InstallK0sBinary installs the k0s binary from the provided file path to K0sBinaryPath
func (h *Host) InstallK0sBinary(path string) error {
	if !h.FS().FileExist(path) {
		return fmt.Errorf("k0s binary tempfile not found")
	}

	dir := h.k0sBinaryPathDir()
	log.Debugf("%s: k0s binary dir: %q", h, dir)
	if err := h.Sudo().FS().MkdirAll(dir, fs.FileMode(0o755)); err != nil {
		return fmt.Errorf("create k0s binary dir: %w", err)
	}
	// Best-effort permissions on POSIX; no-op on Windows
	_ = h.SetFileMode(dir, fs.FileMode(0o755))

	if err := h.Sudo().FS().Rename(path, h.K0sInstallLocation()); err != nil {
		return fmt.Errorf("install k0s binary: %w", err)
	}
	_ = h.SetFileMode(h.K0sInstallLocation(), fs.FileMode(0o750))

	if h.FS().FileExist(path) {
		if err := h.Sudo().FS().Remove(path); err != nil {
			log.Warnf("%s: failed to delete k0s binary tempfile: %s", h, err)
		}
	}

	return nil
}

func (h *Host) K0sBinaryVersion() (*version.Version, error) {
	cmd := h.Configurer.K0sCmdf("version")
	output, err := h.Sudo().ExecOutput(cmd)
	if err != nil {
		return nil, err
	}

	ver, err := version.NewVersion(strings.TrimSpace(output))
	if err != nil {
		return nil, err
	}

	return ver, nil
}

func (h *Host) SetFileMode(path string, mode fs.FileMode) error {
	return h.Sudo().FS().Chmod(path, mode)
}

// UpdateK0sBinary updates the binary on the host from the provided file path
func (h *Host) UpdateK0sBinary(path string, version *version.Version) error {
	if err := h.InstallK0sBinary(path); err != nil {
		return fmt.Errorf("update k0s binary: %w", err)
	}

	updatedVersion, err := h.K0sBinaryVersion()
	if err != nil {
		return fmt.Errorf("failed to get updated k0s binary version: %w", err)
	}
	// verify the installed version matches the expected version, unless a custom k0sbinarypath is used
	if h.K0sBinaryPath == "" && !version.Equal(updatedVersion) {
		return fmt.Errorf("updated k0s binary version is %s not %s", updatedVersion, version)
	}

	h.Metadata.K0sBinaryVersion = version

	return nil
}

// K0sDataDir returns the data dir for the host either from host.DataDir or the default from configurer's DataDirDefaultPath
func (h *Host) K0sDataDir() string {
	if h.DataDir == "" {
		return h.Configurer.DataDirDefaultPath()
	}
	return h.DataDir
}

// DrainNode drains the given node
func (h *Host) DrainNode(node *Host, options DrainOption) error {
	return h.Sudo().Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "drain %s %s", options.ToKubectlArgs(h.Configurer), quote(h.Configurer, node.Metadata.Hostname)))
}

// CordonNode marks the node unschedulable
func (h *Host) CordonNode(node *Host) error {
	return h.Sudo().Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "cordon %s", quote(h.Configurer, node.Metadata.Hostname)))
}

// UncordonNode marks the node schedulable
func (h *Host) UncordonNode(node *Host) error {
	return h.Sudo().Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "uncordon %s", quote(h.Configurer, node.Metadata.Hostname)))
}

// DeleteNode deletes the given node from kubernetes
func (h *Host) DeleteNode(node *Host) error {
	return h.Sudo().Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "delete node %s", quote(h.Configurer, node.Metadata.Hostname)))
}

// Taints returns all taints added to the node.
func (h *Host) Taints(node *Host) ([]string, error) {
	output, err := h.Sudo().ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), `get node %s -o jsonpath='{range .spec.taints[*]}{.key}={.value}:{.effect}{"\n"}{end}'`, quote(h.Configurer, node.Metadata.Hostname)))
	if err != nil {
		return nil, fmt.Errorf("failed to get node taints: %w", err)
	}
	return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// AddTaint adds a taint to the node.
func (h *Host) AddTaint(node *Host, taint string) error {
	return h.Sudo().Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "taint nodes --overwrite %s %s", quote(h.Configurer, node.Metadata.Hostname), quote(h.Configurer, taint)))
}

// RemoveTaint removes a taint from the node.
func (h *Host) RemoveTaint(node *Host, taint string) error {
	tainted, err := h.Taints(node)
	if err != nil {
		return err
	}
	if !slices.Contains(tainted, taint) {
		// Removing a taint not on the node results in an error, so no action is taken
		return nil
	}
	return h.Sudo().Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "taint nodes %s %s-", quote(h.Configurer, node.Metadata.Hostname), quote(h.Configurer, taint)))
}

// CheckHTTPStatus will perform a web request to the url and return an error if the http status is not the expected
func (h *Host) CheckHTTPStatus(url string, expected ...int) error {
	status, err := h.Configurer.HTTPStatus(h, url)
	if err != nil {
		return err
	}

	if slices.Contains(expected, status) {
		return nil
	}

	return fmt.Errorf("expected response code %d but received %d", expected, status)
}

// NeedCurl returns true when the curl package is needed on the host
func (h *Host) NeedCurl() bool {
	// Windows does not need any packages for web requests
	if h.Configurer.Kind() == "windows" {
		return false
	}

	return !h.FS().CommandExist("curl")
}

// NeedIPTables returns true when the iptables package is needed on the host
//
// Deprecated: iptables is only required for k0s versions that are unsupported
// for a long time already (< v1.22.1+k0s.0).
func (h *Host) NeedIPTables() bool {
	// Windows does not need iptables
	if h.Configurer.Kind() == "windows" {
		return false
	}

	// Controllers do not need iptables
	if h.IsController() {
		return false
	}

	return !h.FS().CommandExist("iptables")
}

// NeedInetUtils returns true when the inetutils package is needed on the host to run `hostname`.
func (h *Host) NeedInetUtils() bool {
	// Windows does not need inetutils
	if h.Configurer.Kind() == "windows" {
		return false
	}

	return !h.FS().CommandExist("hostname")
}

// FileChanged returns true when a remote file has different size or mtime compared to local
// or if an error occurs
func (h *Host) FileChanged(lpath, rpath string) bool {
	lstat, err := gos.Stat(lpath)
	if err != nil {
		log.Debugf("%s: local stat failed: %s", h, err)
		return true
	}
	rstat, err := h.Sudo().FS().Stat(rpath)
	if err != nil {
		log.Debugf("%s: remote stat failed: %s", h, err)
		return true
	}

	if lstat.Size() != rstat.Size() {
		log.Debugf("%s: file sizes for %s differ (%d vs %d)", h, lpath, lstat.Size(), rstat.Size())
		return true
	}

	if !lstat.ModTime().Equal(rstat.ModTime()) {
		log.Debugf("%s: file modtimes for %s differ (%s vs %s)", h, lpath, lstat.ModTime(), rstat.ModTime())
		return true
	}

	return false
}

// ExpandTokens expands percent-sign prefixed tokens in a string, mainly for the download URLs.
// The supported tokens are:
//
//   - %% - literal %
//   - %p - host architecture (arm, arm64, amd64, riscv64)
//   - %v - k0s version (v1.21.0+k0s.0)
//   - %x - k0s binary extension (.exe on Windows)
//
// Any unknown token is output as-is with the leading % included.
func (h *Host) ExpandTokens(input string, k0sVersion *version.Version) string {
	if input == "" {
		return ""
	}
	archToken := h.Metadata.Arch
	if archToken == "" {
		if arch, err := h.Arch(); err == nil {
			archToken = arch
		} else {
			log.Warnf("%s: failed to resolve architecture for token expansion: %v", h, err)
		}
	}
	builder := strings.Builder{}
	var inPercent bool
	for i := 0; i < len(input); i++ {
		currCh := input[i]
		if inPercent {
			inPercent = false
			switch currCh {
			case '%':
				// Literal %.
				builder.WriteByte('%')
			case 'p':
				// Host architecture (arm, arm64, amd64, riscv64).
				builder.WriteString(archToken)
			case 'v':
				// K0s version (v1.21.0+k0s.0)
				builder.WriteString(url.QueryEscape(k0sVersion.String()))
			case 'x':
				// K0s binary extension (.exe on Windows).
				if h.Client != nil && h.IsConnected() && h.IsWindows() {
					builder.WriteString(".exe")
				}
			default:
				// Unknown token, just output it with the leading %.
				builder.WriteByte('%')
				builder.WriteByte(currCh)
			}
		} else if currCh == '%' {
			inPercent = true
		} else {
			builder.WriteByte(currCh)
		}
	}
	if inPercent {
		// Trailing %.
		builder.WriteByte('%')
	}
	return builder.String()
}

// FlagsChanged returns true when the flags have changed by comparing the host.Metadata.K0sStatusArgs to what host.InstallFlags would produce
func (h *Host) FlagsChanged() bool {
	our, err := h.K0sInstallFlags()
	if err != nil {
		log.Warnf("%s: could not get install flags: %s", h, err)
		our = Flags{}
	}
	ex := our.GetValue("--kubelet-extra-args")
	ourExtra, err := NewFlags(ex)
	if err != nil {
		log.Warnf("%s: could not parse local --kubelet-extra-args value %q: %s", h, ex, err)
	}

	var their Flags
	their = append(their, h.Metadata.K0sStatusArgs...)
	ex = their.GetValue("--kubelet-extra-args")
	theirExtra, err := NewFlags(ex)
	if err != nil {
		log.Warnf("%s: could not parse remote --kubelet-extra-args value %q: %s", h, ex, err)
	}

	if !ourExtra.Equals(theirExtra) {
		log.Debugf("%s: installFlags --kubelet-extra-args seem to have changed: %+v vs %+v", h, theirExtra.Map(), ourExtra.Map())
		return true
	}

	// remove flags that are dropped by k0s or are handled specially
	for _, f := range []string{"--force", "--kubelet-extra-args", "--env", "--data-dir", "--token-file", "--config"} {
		our.Delete(f)
		their.Delete(f)
	}

	if our.Equals(their) {
		log.Debugf("%s: installFlags have not changed", h)
		return false
	}

	log.Debugf("%s: installFlags seem to have changed. existing: %+v new: %+v", h, their.Map(), our.Map())
	return true
}

// HasHooks returns true when the host has hooks defined for the action and stage.
func (h *Host) HasHooks(action, stage string) bool {
	return len(h.Hooks.ForActionAndStage(action, stage)) > 0
}

// RunHooks runs the hooks for the given action and stage (such as "apply", "before" would run the "before apply" hooks).
// It respects context cancellation between hook executions.
func (h *Host) RunHooks(ctx context.Context, action, stage string) error {
	commands := h.Hooks.ForActionAndStage(action, stage)
	if len(commands) == 0 {
		return nil
	}
	for _, cmd := range commands {
		// Abort early if the context has been canceled.
		if err := ctx.Err(); err != nil {
			return err
		}

		log.Infof("%s: running %s %s hook: %q", h, stage, action, cmd)
		if err := h.Exec(cmd); err != nil {
			return fmt.Errorf("failed to execute hook %q for action %q stage %q on host %s: %w", cmd, action, stage, h.Address(), err)
		}
	}
	return nil
}
