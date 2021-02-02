package cluster

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/creasty/defaults"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
)

// Host contains all the needed details to work with hosts
type Host struct {
	rig.Connection `yaml:",inline"`

	Role             string            `yaml:"role" validate:"oneof=server worker server+worker"`
	PrivateInterface string            `yaml:"privateInterface,omitempty"`
	PrivateAddress   string            `yaml:"privateAddress,omitempty" validate:"omitempty,ip"`
	Environment      map[string]string `yaml:"environment,flow,omitempty" default:"{}"`
	UploadBinary     bool              `yaml:"uploadBinary"`
	K0sBinaryPath    string            `yaml:"k0sBinaryPath,omitempty"`
	InstallFlags     Flags             `yaml:"installFlags,omitempty"`

	Metadata   HostMetadata `yaml:"-"`
	Configurer configurer
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
	K0sConfigPath() string
	K0sJoinTokenPath() string
	WriteFile(os.Host, string, string, string) error
	UpdateEnvironment(os.Host, map[string]string) error
	DaemonReload(os.Host) error
	ReplaceK0sTokenPath(os.Host, string) error
	ServiceScriptPath(os.Host, string) (string, error)
	ReadFile(os.Host, string) (string, error)
	FileExist(os.Host, string) bool
	Chmod(os.Host, string, string) error
	DownloadK0s(os.Host, string, string) error
	WebRequestPackage() string
	InstallPackage(os.Host, ...string) error
	FileContains(os.Host, string, string) bool
	MoveFile(os.Host, string, string) error
	DeleteFile(os.Host, string) error
	CommandExist(os.Host, string) bool
	Hostname(os.Host) string
	InstallKubectl(os.Host) error
	KubectlCmdf(string, ...interface{}) string
	KubeconfigPath() string
	IsContainer(os.Host) bool
	FixContainer(os.Host) error
	HTTPStatus(os.Host, string) (int, error)
	PrivateInterface(os.Host) (string, error)
	PrivateAddress(os.Host, string, string) (string, error)
}

// HostMetadata resolved metadata for host
type HostMetadata struct {
	K0sBinaryVersion  string
	K0sRunningVersion string
	Arch              string
	IsK0sLeader       bool
	Hostname          string
	Ready             bool
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

// Address returns an address for the host
func (h *Host) Address() string {
	if h.SSH != nil {
		return h.SSH.Address
	}

	if h.WinRM != nil {
		return h.WinRM.Address
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
	bf, err := registry.GetOSModuleBuilder(h.OSVersion)
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

// K0sInstallCommand returns a full command that will install k0s service with necessary flags
func (h *Host) K0sInstallCommand() string {
	role := h.Role
	flags := h.InstallFlags

	if role == "server+worker" {
		role = "server"
		flags.AddUnlessExist("--enable-worker")
	}

	if !h.Metadata.IsK0sLeader {
		flags.AddUnlessExist(fmt.Sprintf(`--token-file "%s"`, h.K0sJoinTokenPath()))
	}

	flags.AddUnlessExist(fmt.Sprintf(`--config "%s"`, h.K0sConfigPath()))

	return h.Configurer.K0sCmdf("install %s %s", role, flags.Join())
}

// IsController returns true for server and server+worker roles
func (h *Host) IsController() bool {
	return h.Role == "server" || h.Role == "server+worker"
}

// K0sServiceName returns correct service name
func (h *Host) K0sServiceName() string {
	if h.Role == "server+worker" {
		return "k0sserver"
	}
	return "k0s" + h.Role
}

type kubeNodeStatus struct {
	Items []struct {
		Status struct {
			Conditions []struct {
				Status string `json:"status"`
				Type   string `json:"type"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

// KubeNodeReady runs kubectl on the host and returns true if the given node is marked as ready
func (h *Host) KubeNodeReady(node *Host) (bool, error) {
	output, err := h.ExecOutput(h.Configurer.KubectlCmdf("get node -l kubernetes.io/hostname=%s -o json", node.Metadata.Hostname), exec.HideOutput())
	if err != nil {
		return false, err
	}
	status := kubeNodeStatus{}
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return false, fmt.Errorf("failed to decode kubectl output: %s", err.Error())
	}
	for _, i := range status.Items {
		for _, c := range i.Status.Conditions {
			if c.Type == "Ready" {
				return c.Status == "True", nil
			}
		}
	}

	return false, fmt.Errorf("failed to parse status from kubectl output")
}

// WaitKubeNodeReady blocks until node becomes ready. TODO should probably use Context
func (h *Host) WaitKubeNodeReady(node *Host) error {
	return retry.Do(
		func() error {
			status, err := h.KubeNodeReady(node)
			if err != nil {
				return err
			}
			if !status {
				return fmt.Errorf("%s: node %s did not become ready", h, node.Metadata.Hostname)
			}
			return nil
		},
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.MaxJitter(time.Second*2),
		retry.Delay(time.Second*3),
		retry.Attempts(60),
	)
}

// CheckHTTPStatus will perform a web request to the url and return an error if the http status is not the expected
func (h *Host) CheckHTTPStatus(url string, expected int) error {
	status, err := h.Configurer.HTTPStatus(h, url)
	if err != nil {
		return err
	}

	if status != expected {
		return fmt.Errorf("expected response code %d but received %d", expected, status)
	}

	return nil
}

// WaitHTTPStatus waits until http status received for a GET from the URL is the expected one
func (h *Host) WaitHTTPStatus(url string, expected int) error {
	return retry.Do(
		func() error {
			return h.CheckHTTPStatus(url, expected)
		},
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.MaxJitter(time.Second*2),
		retry.Delay(time.Second*3),
		retry.Attempts(60),
	)
}
