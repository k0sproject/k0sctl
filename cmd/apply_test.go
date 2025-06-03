package cmd

import (
	"flag"
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/urfave/cli/v2"
)

func TestGetNoDrainFlagOrConfig(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.Bool("no-drain", false, "test flag")

	app := cli.NewApp()
	ctx := cli.NewContext(app, set, nil)

	cfg := cluster.DrainOption{Enabled: true}
	if got := getNoDrainFlagOrConfig(ctx, cfg); got {
		t.Errorf("Expected false when config.Enabled is true and flag not set, got true")
	}

	cfg.Enabled = false
	if got := getNoDrainFlagOrConfig(ctx, cfg); !got {
		t.Errorf("Expected true when config.Enabled is false and flag not set, got false")
	}

	_ = set.Set("no-drain", "true")
	if got := getNoDrainFlagOrConfig(ctx, cfg); !got {
		t.Errorf("Expected true when flag is set to true, got false")
	}

	_ = set.Set("no-drain", "false")
	if got := getNoDrainFlagOrConfig(ctx, cfg); got {
		t.Errorf("Expected false when flag is set to false, got true")
	}
}
