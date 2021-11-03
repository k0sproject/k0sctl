package v1beta1

import (
	"fmt"

	validator "github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-version"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	log "github.com/sirupsen/logrus"
)

// APIVersion is the current api version
const APIVersion = "k0sctl.k0sproject.io/v1beta1"

// ClusterMetadata defines cluster metadata
type ClusterMetadata struct {
	Name string `yaml:"name" validate:"required" default:"k0s-cluster"`
}

// Cluster describes launchpad.yaml configuration
type Cluster struct {
	APIVersion string           `yaml:"apiVersion" validate:"required,apiversionmatch"`
	Kind       string           `yaml:"kind" validate:"required,oneof=cluster Cluster"`
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
	validator.RegisterStructValidation(validateHostCounts, cluster.Spec{})
	return validator.Struct(c)
}

func validateHostCounts(sl validator.StructLevel) {
	spec, ok := sl.Current().Interface().(cluster.Spec)
	if !ok {
		return
	}

	if len(spec.Hosts) == 0 {
		sl.ReportError(spec, "hosts", "", "no hosts in configuration", "")
		return
	}

	var hasSingle bool
	for _, h := range spec.Hosts {
		if h.InstallFlags.Get("--single") != "" && h.Role != "single" {
			log.Warnf("%s: changed role from '%s' to 'single' because '--single' defined in installFlags", h, h.Role)
			h.Role = "single"
		}

		if h.InstallFlags.Get("--enable-worker") != "" && h.Role != "controller+worker" {
			log.Warnf("%s: changed role from '%s' to 'controller+worker' because '--enable-workloads' defined in installFlags", h, h.Role)
			h.Role = "controller+worker"
		}

		if h.Role == "single" {
			hasSingle = true
			break
		}
	}

	if hasSingle && len(spec.Hosts) > 1 {
		sl.ReportError(spec, "hosts", "", "contains a host with role: single but multiple hosts defined", "")
	}

	if len(spec.Hosts.Controllers()) == 0 {
		sl.ReportError(spec, "hosts", "", "contains no controller nodes", "")
	}
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
