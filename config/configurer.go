package config

import (
	"github.com/k0sproject/k0sctl/config/generic"
)

// HostConfigurer defines the interface each host OS specific configurers implement.
// This is under api because it has direct deps to api structs
type HostConfigurer interface {
	Arch() (string, error)
	CheckPrivilege() error
	InstallK0sBasePackages() error
	UpdateEnvironment(map[string]string) error
	CleanupEnvironment(map[string]string) error
	K0sConfigPath() string
	K0sJoinTokenPath() string
	K0sBinaryPath() string
	K0sCmdf(...string) string
	UploadK0s(version string, k0sConfig *generic.GenericHash) error
	ValidateFacts() error
	WriteFile(path, content, permissions string) error
	WriteFileLarge(content, permissions string) error
	ReadFile(path string) (string, error)
	DeleteFile(path string) error
	FileExist(path string) bool
	K0sExecutableVersion() (string, error)
	RunK0sDownloader(string) error
	InitSystem() (InitSystem, error)
}
