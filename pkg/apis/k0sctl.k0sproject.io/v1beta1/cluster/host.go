package cluster

import (
	"fmt"
	"net/url"
	gos "os"
	gopath "path"
	"strings"
	"time"

	"al.essio.dev/pkg/shellescape"
	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"github.com/jellydator/validation"
	"github.com/jellydator/validation/is"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

var K0sForceFlagSince = version.MustParse("v1.27.4+k0s.0")

// Host contains all the needed details to work with hosts
type Host struct {
	rig.Connection `yaml:",inline"`

	Role             string            `yaml:"role"`
	Reset            bool              `yaml:"reset,omitempty"`
	PrivateInterface string            `yaml:"privateInterface,omitempty"`
	PrivateAddress   string            `yaml:"privateAddress,omitempty"`
	DataDir          string            `yaml:"dataDir,omitempty"`
	Environment      map[string]string `yaml:"environment,flow,omitempty"`
	UploadBinary     bool              `yaml:"uploadBinary,omitempty"`
	K0sBinaryPath    string            `yaml:"k0sBinaryPath,omitempty"`
	K0sDownloadURL   string            `yaml:"k0sDownloadURL,omitempty"`
	InstallFlags     Flags             `yaml:"installFlags,omitempty"`
	Files            []*UploadFile     `yaml:"files,omitempty"`
	OSIDOverride     string            `yaml:"os,omitempty"`
	HostnameOverride string            `yaml:"hostname,omitempty"`
	NoTaints         bool              `yaml:"noTaints,omitempty"`
	Hooks            Hooks             `yaml:"hooks,omitempty"`

	UploadBinaryPath string       `yaml:"-"`
	Metadata         HostMetadata `yaml:"-"`
	Configurer       configurer   `yaml:"-"`
}

func (h *Host) SetDefaults() {
	if h.OSIDOverride != "" {
		h.OSVersion = &rig.OSVersion{ID: h.OSIDOverride}
	}

	_ = defaults.Set(h.Connection)

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
	// For rig validation
	v := validator.New()
	if err := v.Struct(h); err != nil {
		return err
	}

	return validation.ValidateStruct(h,
		validation.Field(&h.Role, validation.In("controller", "worker", "controller+worker", "single").Error("unknown role "+h.Role)),
		validation.Field(&h.PrivateAddress, is.IP),
		validation.Field(&h.Files),
		validation.Field(&h.NoTaints, validation.When(h.Role != "controller+worker", validation.NotIn(true).Error("noTaints can only be true for controller+worker role"))),
		validation.Field(&h.InstallFlags, validation.Each(validation.By(validateBalancedQuotes))),
	)
}

type configurer interface {
	Kind() string
	CheckPrivilege(os.Host) error
	StartService(os.Host, string) error
	StopService(os.Host, string) error
	RestartService(os.Host, string) error
	ServiceIsRunning(os.Host, string) bool
	Arch(os.Host) (string, error)
	K0sCmdf(string, ...interface{}) string
	K0sBinaryPath() string
	K0sBinaryVersion(os.Host) (*version.Version, error)
	K0sConfigPath() string
	DataDirDefaultPath() string
	K0sJoinTokenPath() string
	WriteFile(os.Host, string, string, string) error
	UpdateEnvironment(os.Host, map[string]string) error
	DaemonReload(os.Host) error
	ReplaceK0sTokenPath(os.Host, string) error
	ServiceScriptPath(os.Host, string) (string, error)
	ReadFile(os.Host, string) (string, error)
	FileExist(os.Host, string) bool
	Chmod(os.Host, string, string, ...exec.Option) error
	DownloadK0s(os.Host, string, *version.Version, string, ...exec.Option) error
	DownloadURL(os.Host, string, string, ...exec.Option) error
	InstallPackage(os.Host, ...string) error
	FileContains(os.Host, string, string) bool
	MoveFile(os.Host, string, string) error
	MkDir(os.Host, string, ...exec.Option) error
	DeleteFile(os.Host, string) error
	CommandExist(os.Host, string) bool
	Hostname(os.Host) string
	KubectlCmdf(os.Host, string, string, ...interface{}) string
	KubeconfigPath(os.Host, string) string
	IsContainer(os.Host) bool
	FixContainer(os.Host) error
	HTTPStatus(os.Host, string) (int, error)
	PrivateInterface(os.Host) (string, error)
	PrivateAddress(os.Host, string, string) (string, error)
	TempDir(os.Host) (string, error)
	TempFile(os.Host) (string, error)
	UpdateServiceEnvironment(os.Host, string, map[string]string) error
	CleanupServiceEnvironment(os.Host, string) error
	Stat(os.Host, string, ...exec.Option) (*os.FileInfo, error)
	Touch(os.Host, string, time.Time, ...exec.Option) error
	DeleteDir(os.Host, string, ...exec.Option) error
	K0sctlLockFilePath(os.Host) string
	UpsertFile(os.Host, string, string) error
	MachineID(os.Host) (string, error)
	SetPath(string, string)
	SystemTime(os.Host) (time.Time, error)
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

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (h *Host) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type host Host
	yh := (*host)(h)

	yh.Environment = make(map[string]string)

	if err := unmarshal(yh); err != nil {
		return err
	}

	if h.SSH != nil && h.SSH.HostKey != "" {
		log.Warnf("%s: host.ssh.hostKey is deprecated, use a ssh known hosts file instead", h)
	}

	return defaults.Set(h)
}

// Address returns an address for the host
func (h *Host) Address() string {
	if addr := h.Connection.Address(); addr != "" {
		return addr
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

	if h.Localhost != nil {
		return "local"
	}

	return "nil"
}

// ResolveConfigurer assigns a rig-style configurer to the Host (see configurer/)
func (h *Host) ResolveConfigurer() error {
	bf, err := registry.GetOSModuleBuilder(*h.OSVersion)
	if err != nil {
		return err
	}

	if c, ok := bf().(configurer); ok {
		h.Configurer = c

		return nil
	}

	return fmt.Errorf("unsupported OS")
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

	flags.AddOrReplace(fmt.Sprintf("--data-dir=%s", shellescape.Quote(h.K0sDataDir())))

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
		flags.AddUnlessExist(fmt.Sprintf(`--token-file=%s`, shellescape.Quote(h.K0sJoinTokenPath())))
	}

	if h.IsController() {
		flags.AddUnlessExist(fmt.Sprintf(`--config=%s`, shellescape.Quote(h.K0sConfigPath())))
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
			flags.AddOrReplace(fmt.Sprintf("--kubelet-extra-args=%s", shellescape.Quote(extra.Join())))
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

	return h.Configurer.K0sCmdf("install %s %s", h.K0sRole(), flags.Join()), nil
}

// K0sBackupCommand returns a full command to be used as run k0s backup
func (h *Host) K0sBackupCommand(targetDir string) string {
	return h.Configurer.K0sCmdf("backup --save-path %s --data-dir %s", shellescape.Quote(targetDir), h.K0sDataDir())
}

// K0sRestoreCommand returns a full command to restore cluster state from a backup
func (h *Host) K0sRestoreCommand(backupfile string) string {
	return h.Configurer.K0sCmdf("restore --data-dir=%s %s", h.K0sDataDir(), shellescape.Quote(backupfile))
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
	return gopath.Dir(h.Configurer.K0sBinaryPath())
}

// InstallK0sBinary installs the k0s binary from the provided file path to K0sBinaryPath
func (h *Host) InstallK0sBinary(path string) error {
	if !h.Configurer.FileExist(h, path) {
		return fmt.Errorf("k0s binary tempfile not found")
	}

	dir := h.k0sBinaryPathDir()
	if err := h.Execf(`install -m 0755 -o root -g root -d "%s"`, dir, exec.Sudo(h)); err != nil {
		return fmt.Errorf("create k0s binary dir: %w", err)
	}

	if err := h.Execf(`install -m 0750 -o root -g root "%s" "%s"`, path, h.Configurer.K0sBinaryPath(), exec.Sudo(h)); err != nil {
		return fmt.Errorf("install k0s binary: %w", err)
	}

	if err := h.Configurer.DeleteFile(h, path); err != nil {
		log.Warnf("%s: failed to delete k0s binary tempfile: %s", h, err)
	}

	return nil
}

// UpdateK0sBinary updates the binary on the host from the provided file path
func (h *Host) UpdateK0sBinary(path string, version *version.Version) error {
	if err := h.InstallK0sBinary(path); err != nil {
		return fmt.Errorf("update k0s binary: %w", err)
	}

	updatedVersion, err := h.Configurer.K0sBinaryVersion(h)
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
func (h *Host) DrainNode(node *Host) error {
	return h.Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "drain --grace-period=120 --force --timeout=5m --ignore-daemonsets --delete-emptydir-data %s", node.Metadata.Hostname), exec.Sudo(h))
}

// CordonNode marks the node unschedulable
func (h *Host) CordonNode(node *Host) error {
	return h.Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "cordon %s", node.Metadata.Hostname), exec.Sudo(h))
}

// UncordonNode marks the node schedulable
func (h *Host) UncordonNode(node *Host) error {
	return h.Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "uncordon %s", node.Metadata.Hostname), exec.Sudo(h))
}

// DeleteNode deletes the given node from kubernetes
func (h *Host) DeleteNode(node *Host) error {
	return h.Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "delete node %s", node.Metadata.Hostname), exec.Sudo(h))
}

// CheckHTTPStatus will perform a web request to the url and return an error if the http status is not the expected
func (h *Host) CheckHTTPStatus(url string, expected ...int) error {
	status, err := h.Configurer.HTTPStatus(h, url)
	if err != nil {
		return err
	}

	for _, e := range expected {
		if status == e {
			return nil
		}
	}

	return fmt.Errorf("expected response code %d but received %d", expected, status)
}

// NeedCurl returns true when the curl package is needed on the host
func (h *Host) NeedCurl() bool {
	// Windows does not need any packages for web requests
	if h.Configurer.Kind() == "windows" {
		return false
	}

	return !h.Configurer.CommandExist(h, "curl")
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

	return !h.Configurer.CommandExist(h, "iptables")
}

// NeedInetUtils returns true when the inetutils package is needed on the host to run `hostname`.
func (h *Host) NeedInetUtils() bool {
	// Windows does not need inetutils
	if h.Configurer.Kind() == "windows" {
		return false
	}

	return !h.Configurer.CommandExist(h, "hostname")
}

// FileChanged returns true when a remote file has different size or mtime compared to local
// or if an error occurs
func (h *Host) FileChanged(lpath, rpath string) bool {
	lstat, err := gos.Stat(lpath)
	if err != nil {
		log.Debugf("%s: local stat failed: %s", h, err)
		return true
	}
	rstat, err := h.Configurer.Stat(h, rpath, exec.Sudo(h))
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
//   - %p - host architecture (arm, arm64, amd64)
//   - %v - k0s version (v1.21.0+k0s.0)
//   - %x - k0s binary extension (.exe on Windows)
//
// Any unknown token is output as-is with the leading % included.
func (h *Host) ExpandTokens(input string, k0sVersion *version.Version) string {
	if input == "" {
		return ""
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
				// Host architecture (arm, arm64, amd64).
				builder.WriteString(h.Metadata.Arch)
			case 'v':
				// K0s version (v1.21.0+k0s.0)
				builder.WriteString(url.QueryEscape(k0sVersion.String()))
			case 'x':
				// K0s binary extension (.exe on Windows).
				if h.IsConnected() && h.IsWindows() {
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

// RunHooks runs the hooks for the given action and stage (such as "apply", "before" would run the "before apply" hooks.
func (h *Host) RunHooks(action, stage string) error {
	// Retrieve hook commands for the given action and stage.
	commands := h.Hooks.ForActionAndStage(action, stage)
	if len(commands) == 0 {
		return nil // No hooks to run.
	}
	for _, cmd := range commands {
		log.Infof("%s: running %s %s hook: %q", h, stage, action, cmd)
		// Execute the command on the host.
		// If needed, you can integrate dry-run behavior here.
		if err := h.Exec(cmd); err != nil {
			return fmt.Errorf("failed to execute hook %q for action %q stage %q on host %s: %w", cmd, action, stage, h.Address(), err)
		}
	}
	return nil
}
