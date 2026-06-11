package configurer

// Configurer defines the per-host operations required for managing a host.
type Configurer interface {
	Kind() string
	OSKind() string
	K0sCmdf(string, ...interface{}) string
	K0sBinaryPath() string
	K0sConfigPath() string
	DataDirDefaultPath() string
	K0sJoinTokenPath() string
	UpdateEnvironment(Host, map[string]string) error
	KubectlCmdf(Host, string, string, ...interface{}) string
	KubeconfigPath(Host, string) string
	FixContainer(Host) error
	PrivateInterface(Host) (string, error)
	PrivateAddress(Host, string, string) (string, error)
	K0sctlLockFilePath(Host) string
	SetPath(string, string)
}

// HostValidator allows a Configurer to implement host-specific validation logic.
type HostValidator interface {
	ValidateHost(Host) error
}
