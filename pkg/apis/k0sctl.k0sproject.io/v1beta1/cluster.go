package v1beta1

import (
	"github.com/creasty/defaults"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

// APIVersion is the current api version
const APIVersion = "k0sctl.k0sproject.io/v1beta1"

// ClusterMetadata defines cluster metadata
type ClusterMetadata struct {
	Name       string      `yaml:"name" validate:"required" default:"k0s-cluster"`
	Kubeconfig string      `yaml:"-"`
	Custom     dig.Mapping `yaml:"-"`
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
	c.Metadata = &ClusterMetadata{
		Name:   "k0s-cluster",
		Custom: dig.Mapping{},
	}
	c.Spec = &cluster.Spec{}

	type clusterConfig Cluster
	yc := (*clusterConfig)(c)

	if err := unmarshal(yc); err != nil {
		return err
	}

	return defaults.Set(c)
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
