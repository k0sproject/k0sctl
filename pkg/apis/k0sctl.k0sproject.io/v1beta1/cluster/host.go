package cluster

import (
	"fmt"
	"net/url"
	gos "os"
	gopath "path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alessio/shellescape"
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

var k0sForceFlagSince = version.MustConstraint(">= v1.27.4+k0s.0")

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
}

// HostMetadata resolved metadata for host
type HostMetadata struct {
	K0sBinaryVersion  *version.Version
	K0sBinaryTempFile string
	K0sRunningVersion *version.Version
	K0sInstalled      bool
	K0sExistingConfig string
	K0sNewConfig      string
	K0sJoinToken      string
	K0sJoinTokenID    string
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

// unquote + unescape a string
func unQE(s string) string {
	unq, err := strconv.Unquote(s)
	if err != nil {
		return s
	}

	c := string(s[0])                                           // string was quoted, c now has the quote char
	re := regexp.MustCompile(fmt.Sprintf(`(?:^|[^\\])\\%s`, c)) // replace \" with " (remove escaped quotes inside quoted string)
	return string(re.ReplaceAllString(unq, c))
}

// K0sInstallCommand returns a full command that will install k0s service with necessary flags
func (h *Host) K0sInstallCommand() (string, error) {
	role := h.Role
	flags := h.InstallFlags

	flags.AddOrReplace(fmt.Sprintf("--data-dir=%s", h.K0sDataDir()))

	switch role {
	case "controller+worker":
		role = "controller"
		flags.AddUnlessExist("--enable-worker")
		if h.NoTaints {
			flags.AddUnlessExist("--no-taints")
		}
	case "single":
		role = "controller"
		flags.AddUnlessExist("--single")
	}

	if !h.Metadata.IsK0sLeader {
		flags.AddUnlessExist(fmt.Sprintf(`--token-file "%s"`, h.K0sJoinTokenPath()))
	}

	if h.IsController() {
		flags.AddUnlessExist(fmt.Sprintf(`--config "%s"`, h.K0sConfigPath()))
	}

	if strings.HasSuffix(h.Role, "worker") {
		var extra Flags
		if old := flags.GetValue("--kubelet-extra-args"); old != "" {
			extra = Flags{unQE(old)}
		}
		// set worker's private address to --node-ip in --extra-kubelet-args if cloud ins't enabled
		enableCloudProvider, err := h.InstallFlags.GetBoolean("--enable-cloud-provider")
		if err != nil {
			return "", fmt.Errorf("--enable-cloud-provider flag is set to invalid value: %s. (%v)", h.InstallFlags.GetValue("--enable-cloud-provider"), err)
		}
		if !enableCloudProvider && h.PrivateAddress != "" {
			extra.AddUnlessExist(fmt.Sprintf("--node-ip=%s", h.PrivateAddress))
		}

		if h.HostnameOverride != "" {
			extra.AddOrReplace(fmt.Sprintf("--hostname-override=%s", h.HostnameOverride))
		}
		if extra != nil {
			flags.AddOrReplace(fmt.Sprintf("--kubelet-extra-args=%s", strconv.Quote(extra.Join())))
		}
	}

	if flags.Include("--force") && h.Metadata.K0sBinaryVersion != nil && !k0sForceFlagSince.Check(h.Metadata.K0sBinaryVersion) {
		log.Warnf("%s: k0s version %s does not support the --force flag, ignoring it", h, h.Metadata.K0sBinaryVersion)
		flags.Delete("--force")
	}

	return h.Configurer.K0sCmdf("install %s %s", role, flags.Join()), nil
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
	if !version.Equal(updatedVersion) {
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
