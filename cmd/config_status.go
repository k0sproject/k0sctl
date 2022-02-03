package cmd

import (
	"fmt"
	"os"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/rig/exec"

	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
)

var configStatusCommand = &cli.Command{
	Name:  "status",
	Usage: "Show k0s dynamic config reconciliation events",
	Flags: []cli.Flag{
		configFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		analyticsFlag,
		upgradeCheckFlag,
	},
	Before: actions(initLogging, startCheckUpgrade, initConfig, initAnalytics),
	After:  actions(reportCheckUpgrade, closeAnalytics),
	Action: func(ctx *cli.Context) error {
		if !isatty.IsTerminal(os.Stdout.Fd()) {
			return fmt.Errorf("output is not a terminal")
		}

		if err := analytics.Client.Publish("config-status-start", map[string]interface{}{}); err != nil {
			return err
		}

		c := ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster)
		h := c.Spec.K0sLeader()

		if err := h.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer h.Disconnect()

		if err := h.ResolveConfigurer(); err != nil {
			return err
		}

		output, err := h.ExecOutput(h.Configurer.K0sCmdf("kubectl -n kube-system get event --field-selector invonvedObject.name=k0s"), exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("%s: %w", h, err)
		}
		fmt.Print(output)

		return nil
	},
}
