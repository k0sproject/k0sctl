package config

import (
	"testing"

	"github.com/k0sproject/k0sctl/config/cluster"
	"github.com/stretchr/testify/require"
)

func TestAPIVersionValidation(t *testing.T) {
	cfg := Cluster{
		APIVersion: "wrongversion",
		Kind:       "cluster",
	}

	require.EqualError(t, cfg.Validate(), "Key: 'Cluster.APIVersion' Error:Field validation for 'APIVersion' failed on the 'apiversionmatch' tag")
	cfg.APIVersion = APIVersion
	require.NoError(t, cfg.Validate())
}

func TestK0sVersionValidation(t *testing.T) {
	cfg := Cluster{
		APIVersion: APIVersion,
		Kind:       "cluster",
		Spec: &cluster.Spec{
			K0s: cluster.K0s{
				Version: "0.1.0",
			},
			Hosts: cluster.Hosts{},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "minimum k0s version")
	cfg.Spec.K0s.Version = cluster.K0sMinVersion
	require.NoError(t, cfg.Validate())
}
