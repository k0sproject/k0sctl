package action

import (
	"context"
	"fmt"
	"io"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
)

type ConfigStatus struct {
	Config      *v1beta1.Cluster
	Concurrency int
	Format      string
	Writer      io.Writer
}

func (c ConfigStatus) Run(ctx context.Context) error {
	h := c.Config.Spec.K0sLeader()

	if err := h.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer h.Disconnect()

	if err := h.ResolveConfigurer(); err != nil {
		return err
	}
	format := c.Format
	if format != "" {
		format = "-o " + format
	}

	output, err := h.Sudo().ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "-n kube-system get event --field-selector involvedObject.name=k0s %s", format))
	if err != nil {
		return fmt.Errorf("%s: %w", h, err)
	}
	fmt.Fprintln(c.Writer, output)

	return nil
}
