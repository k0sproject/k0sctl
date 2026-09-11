package phase

import (
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/download"
)

// hostTracker returns a download tracker for a host's URL file sources.
//
// The transfers go through the sudo filesystem, since the destinations are
// commonly root-owned, while the bookkeeping goes through the plain one so that
// it lands in the login user's cache directory and needs no elevated privileges.
//
// Create one per host and reuse it for the host's files: it looks the host's
// cache directory up once and remembers it.
func hostTracker(h *cluster.Host) *download.Tracker {
	return download.NewTracker(h.Sudo().FS(), h.FS(), h.String())
}
