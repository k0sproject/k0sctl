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
