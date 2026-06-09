package configurer

import (
	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/rig/v2/remotefs"
)

// Host is the interface that configurer methods use to interact with a remote
// host. It is satisfied by *rig.Client (and therefore by *cluster.Host, which
// embeds rig.ClientWithConfig).
type Host interface {
	cmd.SimpleRunner
	Sudo() *rig.Client
	FS() remotefs.FS
}
