package cmd

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0sctl/pkg/manifest"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestReadConfigSetsOrigin(t *testing.T) {
	origin := filepath.Join(t.TempDir(), "cluster.yaml")
	clusterYAML := `apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: test
spec:
  hosts:
    - role: controller
      ssh:
        address: 10.0.0.1
`

	mr := &manifest.Reader{}
	require.NoError(t, mr.ParseBytesWithOrigin([]byte(clusterYAML), origin))

	app := cli.NewApp()
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(app, flagSet, nil)
	ctx.Context = context.WithValue(context.Background(), ctxConfigsKey{}, mr)

	cfg, err := readConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, origin, cfg.Origin)
}

func TestReadConfigResolvesUploadFilesRelativeToOrigin(t *testing.T) {
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets", "bin")
	require.NoError(t, os.MkdirAll(assetsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(assetsDir, "script.sh"), []byte("#!/bin/sh\n"), 0o755))

	origin := filepath.Join(dir, "cluster.yaml")
	clusterYAML := `apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: test
spec:
  hosts:
    - role: controller
      ssh:
        address: 10.0.0.1
      files:
        - src: assets/bin/script.sh
          dstDir: /tmp
`
	require.NoError(t, os.WriteFile(origin, []byte(clusterYAML), 0o644))

	mr := &manifest.Reader{}
	require.NoError(t, mr.ParseBytesWithOrigin([]byte(clusterYAML), origin))

	app := cli.NewApp()
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(app, flagSet, nil)
	ctx.Context = context.WithValue(context.Background(), ctxConfigsKey{}, mr)

	cfg, err := readConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, origin, cfg.Origin)
	require.Len(t, cfg.Spec.Hosts, 1)
	require.Len(t, cfg.Spec.Hosts[0].Files, 1)
	require.Equal(t, filepath.ToSlash(assetsDir), cfg.Spec.Hosts[0].Files[0].Base)
	require.Len(t, cfg.Spec.Hosts[0].Files[0].Sources, 1)
	require.Equal(t, "script.sh", cfg.Spec.Hosts[0].Files[0].Sources[0].Path)
}
