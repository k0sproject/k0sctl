package cluster

import (
	"fmt"

	"github.com/creasty/defaults"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/registry"
)

// Host contains all the needed details to work with hosts
type Host struct {
	rig.Connection `yaml:",inline"`

	Role          string            `yaml:"role" validate:"oneof=server worker server+worker"`
	Environment   map[string]string `yaml:"environment,flow,omitempty" default:"{}"`
	UploadBinary  bool              `yaml:"uploadBinary"`
	K0sBinaryPath string            `yaml:"k0sBinaryPath"`
	InstallFlags  Flags             `yaml:"installFlags"`

	Metadata   HostMetadata `yaml:"-"`
	Configurer configurer
}

type configurer interface {
	CheckPrivilege() error
	StartService(string) error
	RestartService(string) error
	ServiceIsRunning(string) bool
	Arch() (string, error)
	K0sCmdf(string, ...interface{}) string
	K0sBinaryPath() string
	K0sConfigPath() string
	K0sJoinTokenPath() string
	WriteFile(path, data, permissions string) error
	UpdateEnvironment(map[string]string) error
	DaemonReload() error
	ReplaceK0sTokenPath(string) error
	ServiceScriptPath(string) (string, error)
	ReadFile(string) (string, error)
	FileExist(string) bool
	Chmod(string, string) error
	DownloadK0s(string, string) error
	WebRequestPackage() string
	InstallPackage(...string) error
	FileContains(path, substring string) bool
	MoveFile(src, dst string) error
	CommandExist(string) bool
}

// HostMetadata resolved metadata for host
type HostMetadata struct {
	K0sBinaryVersion  string
	K0sRunningVersion string
	Arch              string
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (h *Host) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type host Host
	yh := (*host)(h)

	if err := unmarshal(yh); err != nil {
		return err
	}

	return defaults.Set(h)
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
	bf, err := registry.GetOSModuleBuilder(h.OSVersion)
	if err != nil {
		return err
	}
	if c, ok := bf(h).(configurer); ok {
		h.Configurer = c

		return nil
	}

	return fmt.Errorf("unsupported OS")
}

// K0sJoinTokenPath returns the token file path from install flags or configurer
func (h *Host) K0sJoinTokenPath() string {
	if path := h.InstallFlags.GetValue("--token-path"); path != "" {
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

// K0sInstallCommand returns a full command that will install k0s service with necessary flags
func (h *Host) K0sInstallCommand() string {
	role := h.Role
	flags := h.InstallFlags

	if role == "server+worker" {
		role = "server"
		flags.AddUnlessExist("--enable-worker")
	}

	flags.AddUnlessExist(fmt.Sprintf(`--token-file "%s"`, h.K0sJoinTokenPath()))
	flags.AddUnlessExist(fmt.Sprintf(`--config "%s"`, h.K0sConfigPath()))

	return h.Configurer.K0sCmdf("install %s %s", role, flags.Join())
}

// IsController returns true for server and server+worker roles
func (h *Host) IsController() bool {
	return h.Role == "server" || h.Role == "server+worker"
}
