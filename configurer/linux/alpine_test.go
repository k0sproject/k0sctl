package linux

import (
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

func TestAlpineConfigurerInterface(t *testing.T) {
	h := cluster.Host{}
	h.Configurer = &Alpine{}
}
