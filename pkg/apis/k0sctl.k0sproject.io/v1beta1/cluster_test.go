package v1beta1

import (
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
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
				Version: version.MustParse("0.1.0"),
			},
			Hosts: cluster.Hosts{
				&cluster.Host{Role: "controller"},
			},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "minimum supported k0s version")
	cfg.Spec.K0s.Version = version.MustParse(cluster.K0sMinVersion)
	require.NoError(t, cfg.Validate())
}

func TestSingleHostValidation(t *testing.T) {
	// Host rules have to reach a one host cluster the same as any other, which
	// they did not when Hosts.Validate only looked at hosts in the plural.
	cfg := Cluster{
		APIVersion: APIVersion,
		Kind:       "cluster",
		Spec: &cluster.Spec{
			Hosts: cluster.Hosts{
				&cluster.Host{
					Role: "single",
					Files: []*cluster.UploadFile{
						{Data: "hello", DestinationFile: "/tmp/a.txt", Sha256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
					},
				},
			},
		},
	}

	require.ErrorContains(t, cfg.Validate(), "sha256 can not be used with inline data")

	cfg.Spec.Hosts[0].Files[0].Sha256 = ""
	require.NoError(t, cfg.Validate())
}
