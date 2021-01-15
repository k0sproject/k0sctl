package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/exec"
	"github.com/prometheus/common/log"
)

//Hosts are destnation hosts
type Hosts []*Host

// Host contains all the needed details to work with hosts
type Host struct {
	Role         string            `yaml:"role"`
	UploadBinary bool              `yaml:"uploadBinary,omitempty"`
	K0sBinary    string            `yaml:"k0sBinary,omitempty"`
	Environment  map[string]string `yaml:"environment,flow,omitempty" default:"{}"`

	// Address    string         `yaml:"address" validate:"required,hostname|ip"`
	// SSH        *ssh.Client    `yaml:"ssh,omitempty"`
	// Localhost  bool           `yaml:"localhost,omitempty"`
	Connection rig.Connection `yaml:"connection"`
	// WinRM      *winrm.Client  `yaml:"winRM,omitempty"`

	Metadata *HostMetadata `yaml:"-"`

	name string

	// Hooks        common.Hooks      `yaml:"hooks,omitempty" validate:"dive,keys,oneof=apply reset,endkeys,dive,keys,oneof=before after,endkeys,omitempty"`
	InitSystem InitSystem     `yaml:"-"`
	Configurer HostConfigurer `yaml:"-"`
}

// HostMetadata resolved metadata for host
type HostMetadata struct {
	K0sVersion string
	Arch       string
	Os         *rig.OSVersion
}

// Filter returns a filtered list of Hosts. The filter function should return true for hosts matching the criteria.
func (hosts *Hosts) Filter(filter func(h *Host) bool) Hosts {
	result := make(Hosts, 0, len(*hosts))

	for _, h := range *hosts {
		if filter(h) {
			result = append(result, h)
		}
	}

	return result
}

// ParallelEach runs a function on every Host parallelly. The function should return nil or an error.
// Any errors will be concatenated and returned.
func (hosts *Hosts) ParallelEach(filter func(h *Host) error) error {
	var wg sync.WaitGroup
	var errors []string
	type erritem struct {
		address string
		err     error
	}
	ec := make(chan erritem, 1)

	wg.Add(len(*hosts))
	fmt.Println("HOSTS ----->", spew.Sdump(hosts))
	for _, h := range *hosts {
		fmt.Println("HOST ----->", spew.Sdump(h))
		fmt.Println("HOST string ----->", h.String())

		go func(h *Host) {
			ec <- erritem{h.String(), filter(h)}
		}(h)
	}

	go func() {
		for e := range ec {
			if e.err != nil {
				errors = append(errors, fmt.Sprintf("%s: %s", e.address, e.err.Error()))
			}
			wg.Done()
		}
	}()

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("failed on %d hosts:\n - %s", len(errors), strings.Join(errors, "\n - "))
	}

	return nil
}

func (h *Host) String() string {
	if h.name == "" {
		h.name = h.generateName()
	}

	return h.name
}

// Connect connects to the host
func (h *Host) Connect() error {

	return h.Connection.Connect()
}

// Exec - executes a command on a host
func (h *Host) Exec(cmd string, opts ...exec.Option) error {
	return h.Connection.Exec(cmd, opts...)
}

func (h *Host) generateName() string {
	if h.Connection.Localhost != nil {
		return fmt.Sprintf("localhost")
	}

	if h.Connection.WinRM != nil {
		return fmt.Sprintf("%s:%d", h.Connection.WinRM.Address, h.Connection.WinRM.Port)
	}

	if h.Connection.SSH != nil {
		return fmt.Sprintf("%s:%d", h.Connection.SSH.Address, h.Connection.SSH.Port)
	}

	return "none"
}

// ResolveHostConfigurer will resolve and cast a configurer for the K0s configurer interface
func (h *Host) ResolveHostConfigurer() error {
	log.Debugf("CALLED!!!!!!!!!!")
	if h.Metadata == nil || h.Metadata.Os == nil {
		return fmt.Errorf("%s: OS not known", h)
	}

	r, err := rig.GetResolver(&h.Connection)
	if err != nil {
		return err
	}

	if configurer, ok := r.(HostConfigurer); ok {
		h.Configurer = configurer
		return fmt.Errorf("12rawersbkml203r")
	}

	return fmt.Errorf("%s: has unsupported OS (%s)", h, h.Metadata.Os)
}

// ExecWithOutput execs a command on the host and returns output
func (h *Host) ExecWithOutput(cmd string, opts ...exec.Option) (string, error) {
	var output string
	opts = append(opts, exec.Output(&output))
	err := h.Exec(cmd, opts...)
	return strings.TrimSpace(output), err
}

func (h *Host) IsWindows() bool {
	is, _ := h.Connection.IsWindows()

	return is
}
