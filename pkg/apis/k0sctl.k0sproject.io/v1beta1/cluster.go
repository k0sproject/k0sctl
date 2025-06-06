package v1beta1

import (
	"bytes"
	"fmt"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
	"gopkg.in/yaml.v2"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

// APIVersion is the current api version
const APIVersion = "k0sctl.k0sproject.io/v1beta1"

// ClusterMetadata defines cluster metadata
type ClusterMetadata struct {
	Name        string            `yaml:"name" validate:"required" default:"k0s-cluster"`
	User        string            `yaml:"user" default:"admin"`
	Kubeconfig  string            `yaml:"-"`
	EtcdMembers []string          `yaml:"-"`
	Manifests   map[string][]byte `yaml:"-"`
}

// Cluster describes launchpad.yaml configuration
type Cluster struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   *ClusterMetadata `yaml:"metadata"`
	Spec       *cluster.Spec    `yaml:"spec"`
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *Cluster) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Metadata = &ClusterMetadata{}
	c.Spec = &cluster.Spec{}

	type clusterConfig Cluster
	yc := (*clusterConfig)(c)

	if err := unmarshal(yc); err != nil {
		return err
	}

	if err := defaults.Set(c); err != nil {
		return fmt.Errorf("failed to set defaults: %w", err)
	}

	return nil
}

// String renders the config as a string
func (c *Cluster) String() string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(c); err != nil {
		return "# error enconding cluster config: " + err.Error()
	}
	return buf.String()
}

// SetDefaults initializes default values
func (c *Cluster) SetDefaults() {
	if c.Metadata == nil {
		c.Metadata = &ClusterMetadata{}
	}
	if c.Spec == nil {
		c.Spec = &cluster.Spec{}
	}
	_ = defaults.Set(c.Metadata)
	_ = defaults.Set(c.Spec)
	if defaults.CanUpdate(c.APIVersion) {
		c.APIVersion = APIVersion
	}
	if defaults.CanUpdate(c.Kind) {
		c.Kind = "Cluster"
	}
}

// Validate performs a configuration sanity check
func (c *Cluster) Validate() error {
	validation.ErrorTag = "yaml"
	return validation.ValidateStruct(c,
		validation.Field(&c.APIVersion, validation.Required, validation.In(APIVersion).Error("must equal "+APIVersion)),
		validation.Field(&c.Kind, validation.Required, validation.In("cluster", "Cluster").Error("must equal Cluster")),
		validation.Field(&c.Spec),
	)
}

// StorageType returns the k0s storage type.
func (c *Cluster) StorageType() string {
	if c.Spec == nil {
		// default to etcd when there's no hosts or k0s spec, this should never happen.
		return "etcd"
	}

	if c.Spec.K0s != nil {
		if t := c.Spec.K0s.Config.DigString("spec", "storage", "type"); t != "" {
			// if storage type is set in k0s spec, return it
			return t
		}
	}

	if h := c.Spec.K0sLeader(); h != nil && h.Role == "single" {
		// default to "kine" on single node clusters
		return "kine"
	}

	// default to etcd otherwise
	return "etcd"
}
