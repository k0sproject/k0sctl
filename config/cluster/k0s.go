package cluster

// K0s holds configuration for bootstraping a k0s cluster
type K0s struct {
	Version  string      `yaml:"version" default:"0.9.1"`
	Config   Mapping     `yaml:"k0s"`
	Metadata K0sMetadata `yaml:"-"`
}

// K0sMetadata contains gathered information about k0s cluster
type K0sMetadata struct {
	ClusterID       string
	ControllerToken string
	WorkerToken     string
}
