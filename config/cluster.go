package config

import (
	"fmt"

	validator "github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-version"
	"github.com/k0sproject/k0sctl/config/cluster"
)

// APIVersion is the current api version
const APIVersion = "k0sctl.k0sproject.io/v1beta1"

// ClusterMetadata defines cluster metadata
type ClusterMetadata struct {
	Name string `yaml:"name" validate:"required"`
}

// Cluster describes launchpad.yaml configuration
type Cluster struct {
	APIVersion string           `yaml:"apiVersion" validate:"required,apiversionmatch"`
	Kind       string           `yaml:"kind" validate:"required,eq=cluster"`
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
	validator.RegisterStructValidation(validateMinK0sVersion, cluster.K0s{})
	if err := validator.RegisterValidation("apiversionmatch", validateAPIVersion); err != nil {
		return err
	}
	return validator.Struct(c)
}

func validateAPIVersion(fl validator.FieldLevel) bool {
	return fl.Field().String() == APIVersion
}

func validateMinK0sVersion(sl validator.StructLevel) {
	if k0s, ok := sl.Current().Interface().(cluster.K0s); ok {
		v, err := version.NewVersion(k0s.Version)
		if err != nil {
			sl.ReportError(k0s.Version, "version", "", "invalid version", "")
			return
		}
		min, err := version.NewVersion(cluster.K0sMinVersion)
		if err != nil {
			panic("invalid k0s minversion")
		}
		if v.LessThan(min) {
			sl.ReportError(k0s.Version, "version", "", fmt.Sprintf("minimum k0s version is %s", cluster.K0sMinVersion), "")
		}
	}
}
