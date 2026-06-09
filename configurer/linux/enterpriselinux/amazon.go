package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	rigos "github.com/k0sproject/rig/v2/os"
)

// AmazonLinux provides OS support for AmazonLinux
type AmazonLinux struct {
	k0slinux.EnterpriseLinux
}

var _ configurer.Configurer = (*AmazonLinux)(nil)

// Hostname on amazon linux will return the full hostname
func (l *AmazonLinux) Hostname(h configurer.Host) string {
	hostname, _ := h.ExecOutput("hostname")

	return hostname
}

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "amzn"
		},
		func() any {
			return &AmazonLinux{}
		},
	)
}
