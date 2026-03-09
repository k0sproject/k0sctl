package k0s

import "context"

// BinaryProvider knows how to acquire a k0s binary for a specific host.
// Implementations are typically created per-host and may close over host-specific state.
// The five built-in implementations in pkg/k0s/binprovider are binprovider.NewGitHub, binprovider.NewCustomURL,
// binprovider.NewLocalFile, binprovider.NewLocalUpload, and binprovider.NewExisting, selected automatically
// based on the host configuration, but custom implementations can be set via host.SetK0sBinaryProvider.
type BinaryProvider interface {
	// NeedsUpgrade reports whether the host's k0s binary needs to be updated.
	// The provider determines this by comparing its target version against the
	// host's currently installed and running k0s versions, read at call time.
	NeedsUpgrade() bool

	// IsUpload reports whether staging this binary requires an upload from the
	// local machine to the remote host. When true, StageBinaries uses the upload
	// concurrency limit instead of the general concurrency limit.
	IsUpload() bool

	// Stage acquires the binary and returns the path to a temporary file on the host.
	// Returns ("", nil) when the binary is already in-place (e.g. pre-placed by the
	// user), in which case no binary replacement will occur during upgrade — only a
	// service reinstall and restart.
	Stage(ctx context.Context) (string, error)

	// CleanUp removes any temporary files created by Stage.
	CleanUp(ctx context.Context)
}

// BinaryCacher is an optional interface that BinaryProvider implementations may
// implement when staging requires a locally-cached binary (e.g. local-upload providers).
// StageBinaries calls BinaryCacheKey to deduplicate across hosts that need the same
// binary, then calls EnsureCached exactly once per unique key before staging begins.
type BinaryCacher interface {
	// BinaryCacheKey returns a unique identifier for the binary this provider needs.
	// Providers that share the same key map to the same local cache file and only
	// one EnsureCached call will be made for them.
	BinaryCacheKey() (string, error)

	// EnsureCached downloads the binary to the local cache if not already present.
	EnsureCached(ctx context.Context) error
}
