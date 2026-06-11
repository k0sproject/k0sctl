package configurer

import (
	"sync"

	rigos "github.com/k0sproject/rig/v2/os"
)

type registryEntry struct {
	matcher func(*rigos.Release) bool
	builder func() any
}

var (
	registryMu      sync.RWMutex
	registryEntries []registryEntry
)

// RegisterOSModule registers a configurer factory for hosts whose detected OS
// release matches the given predicate. Modules are evaluated in registration
// order and the first matching one wins, so register more specific matchers
// before more general ones.
func RegisterOSModule(matcher func(*rigos.Release) bool, builder func() any) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registryEntries = append(registryEntries, registryEntry{matcher: matcher, builder: builder})
}

// ResolveOSModule returns the factory for the first registered module whose
// matcher accepts the given release. The boolean is false when no module
// matches.
func ResolveOSModule(release *rigos.Release) (func() any, bool) {
	if release == nil {
		return nil, false
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	for _, e := range registryEntries {
		if e.matcher(release) {
			return e.builder, true
		}
	}
	return nil, false
}
