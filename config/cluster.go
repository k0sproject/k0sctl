package config

import (
	validator "github.com/go-playground/validator/v10"
	"github.com/k0sproject/k0sctl/config/cluster"
)

// ClusterMetadata defines cluster metadata
type ClusterMetadata struct {
	Name string `yaml:"name" validate:"required"`
}

// Cluster describes launchpad.yaml configuration
type Cluster struct {
	APIVersion string           `yaml:"apiVersion" validate:"eq=k0sctl.k0sproject.io/v1beta1"`
	Kind       string           `yaml:"kind" validate:"eq=cluster"`
	Metadata   *ClusterMetadata `yaml:"metadata"`
	Spec       *cluster.Spec    `yaml:"spec"`
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *Cluster) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Metadata = &ClusterMetadata{
		Name: "k0s-cluster",
	}
	c.Spec = &cluster.Spec{}

	type clusterConfig Cluster
	yc := (*clusterConfig)(c)

	if err := unmarshal(yc); err != nil {
		return err
	}

	return nil
}

// Validate performs a configuration sanity check
func (c *Cluster) Validate() error {
	validator := validator.New()
	return validator.Struct(c)
}
