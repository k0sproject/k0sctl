package cluster

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostsEach(t *testing.T) {
	hosts := Hosts{
		&Host{Role: "controller"},
		&Host{Role: "worker"},
	}

	t.Run("success", func(t *testing.T) {
		var roles []string
		fn := func(_ context.Context, h *Host) error {
			roles = append(roles, h.Role)
			return nil
		}
		err := hosts.Each(context.Background(), fn)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"controller", "worker"}, roles)
		require.Len(t, roles, 2)
	})

	t.Run("context cancel", func(t *testing.T) {
		var count int
		ctx, cancel := context.WithCancel(context.Background())

		fn := func(ctx context.Context, h *Host) error {
			count++
			cancel()
			return nil
		}
		err := hosts.Each(ctx, fn)
		require.Equal(t, 1, count)
		require.Error(t, err)
		require.ErrorContains(t, err, "cancel")
	})

	t.Run("error", func(t *testing.T) {
		fn := func(_ context.Context, h *Host) error {
			return errors.New("test")
		}
		err := hosts.Each(context.Background(), fn)
		require.Error(t, err)
		require.ErrorContains(t, err, "test")
	})
}

func TestHostsValidateSingleHost(t *testing.T) {
	// A one host cluster used to skip host validation entirely, which let every
	// per-host rule through unchecked on the most ordinary configuration there
	// is.
	t.Run("host rules apply", func(t *testing.T) {
		hosts := Hosts{
			&Host{
				Role: "single",
				Files: []*UploadFile{
					{Source: "https://example.com/a.tar", DestinationFile: "/tmp/a.tar", Sha256: "nothex"},
				},
			},
		}
		require.ErrorContains(t, hosts.Validate(), "must be a hex encoded sha256 sum")
	})

	t.Run("the single role is allowed", func(t *testing.T) {
		hosts := Hosts{&Host{Role: "single"}}
		require.NoError(t, hosts.Validate())
	})

	t.Run("a valid host passes", func(t *testing.T) {
		hosts := Hosts{
			&Host{
				Role: "controller",
				Files: []*UploadFile{
					{Source: "https://example.com/a.tar", DestinationFile: "/tmp/a.tar", Sha256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
				},
			},
		}
		require.NoError(t, hosts.Validate())
	})
}
