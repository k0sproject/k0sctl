package v1beta1

import (
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/stretchr/testify/require"
)

func TestAPIVersionValidation(t *testing.T) {
	cfg := Cluster{
		APIVersion: "wrongversion",
		Kind:       "cluster",
	}

	require.EqualError(t, cfg.Validate(), "apiVersion: must equal k0sctl.k0sproject.io/v1beta1.")
	cfg.APIVersion = APIVersion
	require.NoError(t, cfg.Validate())
}

func TestK0sVersionValidation(t *testing.T) {
	cfg := Cluster{
		APIVersion: APIVersion,
		Kind:       "cluster",
		Spec: &cluster.Spec{
			K0s: &cluster.K0s{
				Version: "0.1.0",
			},
			Hosts: cluster.Hosts{
				&cluster.Host{Role: "controller"},
			},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "minimum supported k0s version")
	cfg.Spec.K0s.Version = cluster.K0sMinVersion
	require.NoError(t, cfg.Validate())
}
