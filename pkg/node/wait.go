package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
)

func retry(ctx context.Context, f func(ctx context.Context) error) error {
	var lastErr error

	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), lastErr)
		case <-ticker.C:
			lastErr = f(ctx)
			if lastErr == nil {
				return nil
			}
		}
	}
}

// WaitKubeNodeReady blocks until node becomes ready or context is cancelled
// TODO this could use "kubectl wait node --for=condition=Ready --timeout=5m"
func WaitKubeNodeReady(ctx context.Context, h *cluster.Host) error {
	return retry(ctx, func(_ context.Context) error {
		status, err := kubeNodeReady(h)
		if err != nil {
			return err
		}
		if !status {
			return fmt.Errorf("%s: node %s status not reported as Ready", h, h.Metadata.Hostname)
		}
		return nil
	})
}

type kubeNodeStatus struct {
	Items []struct {
		Status struct {
			Conditions []struct {
				Status string `json:"status"`
				Type   string `json:"type"`
			} `json:"conditions"`
		} `json:"status"`
	} `json:"items"`
}

func kubeNodeReady(h *cluster.Host) (bool, error) {
	output, err := h.ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get node -l kubernetes.io/hostname=%s -o json", h.Metadata.Hostname), exec.HideOutput(), exec.Sudo(h))
	if err != nil {
		return false, err
	}
	status := kubeNodeStatus{}
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return false, fmt.Errorf("failed to decode kubectl output: %s", err.Error())
	}
	for _, i := range status.Items {
		for _, c := range i.Status.Conditions {
			if c.Type == "Ready" {
				return c.Status == "True", nil
			}
		}
	}

	return false, nil
}

type statusEvents struct {
	Items []struct {
		Reason string `json:"reason"`
	} `json:"items"`
}

// WaitK0sDynamicConfigReady blocks until dynamic config reconciliation has been performed or context is cancelled
func WaitK0sDynamicConfigReady(ctx context.Context, h *cluster.Host) error {
	return retry(ctx, func(_ context.Context) error {
		output, err := h.ExecOutput(h.Configurer.K0sCmdf("kubectl --data-dir=%s -n kube-system get event --field-selector involvedObject.name=k0s -o json", h.K0sDataDir()), exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("failed to get k0s config status events: %w", err)
		}
		events := &statusEvents{}
		if err := json.Unmarshal([]byte(output), &events); err != nil {
			return fmt.Errorf("failed to decode kubectl output: %w", err)
		}
		for _, e := range events.Items {
			if e.Reason == "SuccessfulReconcile" {
				return nil
			}
		}
		return fmt.Errorf("dynamic config not ready")
	})
}

// WaitHTTPStatus waits on the node until http status received for a GET from the URL is the
// expected one or context is cancelled
func WaitHTTPStatus(ctx context.Context, h *cluster.Host, url string, expected ...int) error {
	return retry(ctx, func(_ context.Context) error {
		return h.CheckHTTPStatus(url, expected...)
	})
}

// WaitServiceRunning blocks until the service is running on the host or context is cancelled
func WaitServiceRunning(ctx context.Context, h *cluster.Host, service string) error {
	return retry(ctx, func(_ context.Context) error {
		if !h.Configurer.ServiceIsRunning(h, service) {
			return fmt.Errorf("service %s is not running", service)
		}
		return h.Exec(h.Configurer.K0sCmdf("status"), exec.Sudo(h))
	})
}

// WaitServiceStopped blocks until the k0s service is no longer running on the host or context is cancelled
func WaitServiceStopped(ctx context.Context, h *cluster.Host, service string) error {
	return retry(ctx, func(_ context.Context) error {
		if h.Configurer.ServiceIsRunning(h, service) {
			return fmt.Errorf("service %s is still running", service)
		}
		return nil
	})
}

// KubeAPIReady blocks until the local kube api responds to /version or context is cancelled
func WaitKubeAPIReady(ctx context.Context, h *cluster.Host, port int) error {
	// If the anon-auth is disabled on kube api the version endpoint will give 401
	// thus we need to accept both 200 and 401 as valid statuses when checking kube api
	return WaitHTTPStatus(ctx, h, fmt.Sprintf("https://localhost:%d/version", port), 200, 401)
}
