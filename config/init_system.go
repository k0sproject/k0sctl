package config

import (
	"fmt"

	"github.com/k0sproject/rig/exec"
)

// InitSystem is an interface for the OS init system
type InitSystem interface {
	StartService(string) error
	StopService(string) error
	RestartService(string) error
	EnableService(string) error
	DisableService(string) error
	DaemonReload() error
	ScriptPath(string) (string, error)
	ServiceIsRunning(string) bool
}

type host interface {
	Exec(string, ...exec.Option) error
	ExecWithOutput(string, ...exec.Option) (string, error)
}

// Systemd is an init system implementation for systemd systems
type Systemd struct {
	Host host
}

// OpenRC is an init system implementation for openrc systems
type OpenRC struct {
	Host host
}

// StartService starts a a service
func (i *Systemd) StartService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo systemctl start %s", s))
}

// EnableService enables a a service
func (i *Systemd) EnableService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo systemctl enable %s", s))
}

// DisableService disables a a service
func (i *Systemd) DisableService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo systemctl disable %s", s))
}

// StopService stops a a service
func (i *Systemd) StopService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo systemctl stop %s", s))
}

// RestartService restarts a a service
func (i *Systemd) RestartService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo systemctl restart %s", s))
}

// DaemonReload reloads init system configuration
func (i *Systemd) DaemonReload() error {
	return i.Host.Exec("sudo systemctl daemon-reload")
}

// ServiceIsRunning returns true if a service is running
func (i *Systemd) ServiceIsRunning(s string) bool {
	return i.Host.Exec(fmt.Sprintf(`sudo systemctl status %s | grep -q "(running)"`, s)) == nil
}

// ScriptPath returns the path to a service configuration file
func (i *Systemd) ScriptPath(s string) (string, error) {
	return i.Host.ExecWithOutput(fmt.Sprintf(`systemctl show -p FragmentPath %s.service 2> /dev/null | cut -d"=" -f2)`, s))
}

// StartService starts a a service
func (i *OpenRC) StartService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo rc-service %s start", s))
}

// StopService stops a a service
func (i *OpenRC) StopService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo rc-service %s stop", s))
}

// ScriptPath returns the path to a service configuration file
func (i *OpenRC) ScriptPath(s string) (string, error) {
	return i.Host.ExecWithOutput(fmt.Sprintf("sudo rc-service -r %s 2> /dev/null", s))
}

// RestartService restarts a a service
func (i *OpenRC) RestartService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo rc-service %s restart", s))
}

// DaemonReload reloads init system configuration
func (i *OpenRC) DaemonReload() error {
	return nil
}

// EnableService enables a a service
func (i *OpenRC) EnableService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo rc-update add %s", s))
}

// DisableService disables a a service
func (i *OpenRC) DisableService(s string) error {
	return i.Host.Exec(fmt.Sprintf("sudo rc-update del %s", s))
}

// ServiceIsRunning returns true if a service is running
func (i *OpenRC) ServiceIsRunning(s string) bool {
	return i.Host.Exec(fmt.Sprintf(`sudo rc-service %s status | grep -q "status: started"`, s)) == nil
}
