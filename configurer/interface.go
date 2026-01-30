package configurer

import (
	"time"

	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
)

// Configurer defines the per-host operations required for managing a host.
type Configurer interface {
	Kind() string
	OSKind() string
	Quote(string) string
	CheckPrivilege(os.Host) error
	StartService(os.Host, string) error
	StopService(os.Host, string) error
	RestartService(os.Host, string) error
	ServiceIsRunning(os.Host, string) bool
	Arch(os.Host) (string, error)
	K0sCmdf(string, ...interface{}) string
	K0sBinaryPath() string
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
	Chown(os.Host, string, string, ...exec.Option) error
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
	DeleteDir(os.Host, string, ...exec.Option) error
	K0sctlLockFilePath(os.Host) string
	UpsertFile(os.Host, string, string) error
	MachineID(os.Host) (string, error)
	SetPath(string, string)
	SystemTime(os.Host) (time.Time, error)
	Touch(os.Host, string, time.Time, ...exec.Option) error
	Dir(string) string
	Base(string) string
	HostPath(string) string
	LookPath(os.Host, string) (string, error)
}

// HostValidator allows a Configurer to implement host-specific validation logic.
type HostValidator interface {
	ValidateHost(os.Host) error
}
