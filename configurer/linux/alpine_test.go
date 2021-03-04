package linux

import (
	"testing"

	"github.com/k0sproject/k0sctl/config/cluster"
)

func TestAlpineConfigurerInterface(t *testing.T) {
	h := cluster.Host{}
	h.Configurer = Alpine{}
}
