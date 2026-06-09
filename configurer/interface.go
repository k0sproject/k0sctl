package configurer

import (
	"io/fs"
	"time"
)

// Configurer defines the per-host operations required for managing a host.
type Configurer interface {
	Kind() string
	OSKind() string
	Quote(string) string
	CheckPrivilege(Host) error
	StartService(Host, string) error
	StopService(Host, string) error
	RestartService(Host, string) error
	ServiceIsRunning(Host, string) bool
	Arch(Host) (string, error)
	K0sCmdf(string, ...interface{}) string
	K0sBinaryPath() string
	K0sConfigPath() string
	DataDirDefaultPath() string
	K0sJoinTokenPath() string
	WriteFile(Host, string, string, string) error
	UpdateEnvironment(Host, map[string]string) error
	DaemonReload(Host) error
	ReplaceK0sTokenPath(Host, string) error
	ServiceScriptPath(Host, string) (string, error)
	ReadFile(Host, string) (string, error)
	FileExist(Host, string) bool
	Chmod(Host, string, string) error
	Chown(Host, string, string) error
	DownloadURL(Host, string, string) error
	InstallPackage(Host, ...string) error
	FileContains(Host, string, string) bool
	MoveFile(Host, string, string) error
	MkDir(Host, string) error
	DeleteFile(Host, string) error
	CommandExist(Host, string) bool
	Hostname(Host) string
	KubectlCmdf(Host, string, string, ...interface{}) string
	KubeconfigPath(Host, string) string
	IsContainer(Host) bool
	FixContainer(Host) error
	HTTPStatus(Host, string) (int, error)
	PrivateInterface(Host) (string, error)
	PrivateAddress(Host, string, string) (string, error)
	TempDir(Host) (string, error)
	TempFile(Host) (string, error)
	UpdateServiceEnvironment(Host, string, map[string]string) error
	CleanupServiceEnvironment(Host, string) error
	Stat(Host, string) (fs.FileInfo, error)
	DeleteDir(Host, string) error
	K0sctlLockFilePath(Host) string
	UpsertFile(Host, string, string) error
	MachineID(Host) (string, error)
	SetPath(string, string)
	SystemTime(Host) (time.Time, error)
	Touch(Host, string, time.Time) error
	Dir(string) string
	Base(string) string
	HostPath(string) string
	LookPath(Host, string) (string, error)
}

// HostValidator allows a Configurer to implement host-specific validation logic.
type HostValidator interface {
	ValidateHost(Host) error
}
