package action

import (
	"fmt"
	"io"
	"os"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/rig/exec"

	osexec "os/exec"

	"github.com/mattn/go-isatty"
)

type ConfigEdit struct {
	Config *v1beta1.Cluster
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

func (c ConfigEdit) Run() error {
	stdoutFile, ok := c.Stdout.(*os.File)

	if !ok || !isatty.IsTerminal(stdoutFile.Fd()) {
		return fmt.Errorf("output is not a terminal")
	}

	editor, err := shellEditor()
	if err != nil {
		return err
	}

	h := c.Config.Spec.K0sLeader()

	if err := h.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer h.Disconnect()

	if err := h.ResolveConfigurer(); err != nil {
		return err
	}

	oldCfg, err := h.ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "-n kube-system get clusterconfig k0s -o yaml"), exec.Sudo(h))
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
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
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

	if err := h.Exec(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "apply -n kube-system -f -"), exec.Stdin(newCfg), exec.Sudo(h)); err != nil {
		return err
	}

	return nil
}

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
