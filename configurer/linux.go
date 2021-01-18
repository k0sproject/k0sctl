package configurer

import (
	"github.com/k0sproject/rig/os"
)

// Linux is a base module for various linux OS support packages
type Linux struct {
	os.Linux
}

func (l *Linux) CheckPrivilege() error {
	return l.Linux.CheckPrivilege()
}

func (l *Linux) ServiceIsRunning(s string) bool {
	return l.Linux.ServiceIsRunning(s)
}
