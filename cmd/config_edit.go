package cmd

import (
	"fmt"
	"os"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/rig/exec"

	osexec "os/exec"

	"github.com/mattn/go-isatty"
	"github.com/urfave/cli/v2"
)

func shellEditor() (string, error) {
	if v := os.Getenv("VISUAL"); v != "" {
		return v, nil
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return v, nil
	}
	if path, err := osexec.LookPath("vi"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("could not detect shell editor ($VISUAL, $EDITOR)")
}

var configEditCommand = &cli.Command{
	Name:  "edit",
	Usage: "Edit k0s dynamic config in SHELL's default editor",
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

		analytics.Client.Publish("config-edit-start", map[string]interface{}{})

		editor, err := shellEditor()
		if err != nil {
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

		oldCfg, err := h.ExecOutput(h.Configurer.K0sCmdf("kubectl --data-dir=%s -n kube-system get clusterconfig k0s -o yaml", h.DataDir), exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("%s: %w", h, err)
		}

		tmpFile, err := os.CreateTemp("", "k0s-config.*.yaml")
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.WriteString(oldCfg); err != nil {
			return err
		}

		if err := tmpFile.Close(); err != nil {
			return err
		}

		cmd := osexec.Command(editor, tmpFile.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start editor (%s): %w", cmd.String(), err)
		}

		newCfgBytes, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			return err
		}
		newCfg := string(newCfgBytes)

		if newCfg == oldCfg {
			return fmt.Errorf("configuration was not changed, aborting")
		}

		if err := h.Exec(h.Configurer.K0sCmdf("kubectl apply --data-dir=%s -n kube-system -f -", h.DataDir), exec.Stdin(newCfg), exec.Sudo(h)); err != nil {
			return err
		}

		return nil
	},
}
