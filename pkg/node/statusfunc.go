package node

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"

	log "github.com/sirupsen/logrus"
)

// this file contains functions that return functions that can be used with pkg/retry to wait on certain
// status conditions of nodes

type retryFunc func(context.Context) error

// kubectl get node -o json
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

// kubectl get events -o json
type statusEvents struct {
	Items []struct {
		InvolvedObject struct {
			Name string `json:"name"`
		} `json:"involvedObject"`
		Reason    string    `json:"reason"`
		EventTime time.Time `json:"eventTime"`
	} `json:"items"`
}

// KubeNodeReady returns a function that returns an error unless the node is ready according to "kubectl get node"
func KubeNodeReadyFunc(h *cluster.Host) retryFunc {
	return func(_ context.Context) error {
		output, err := h.ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get node -l kubernetes.io/hostname=%s -o json", strings.ToLower(h.Metadata.Hostname)), exec.HideOutput(), exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("failed to get node status: %w", err)
		}
		status := &kubeNodeStatus{}
		if err := json.Unmarshal([]byte(output), status); err != nil {
			return fmt.Errorf("failed to decode kubectl get node status output: %w", err)
		}
		for _, i := range status.Items {
			for _, c := range i.Status.Conditions {
				if c.Type == "Ready" {
					if c.Status == "True" {
						return nil
					}
					return fmt.Errorf("node %s is not ready", h.Metadata.Hostname)
				}
			}
		}
		return fmt.Errorf("node %s 'Ready' condition not found", h.Metadata.Hostname)
	}
}

// K0sDynamicConfigReadyFunc returns a function that returns an error unless the k0s dynamic config has been reconciled
func K0sDynamicConfigReadyFunc(h *cluster.Host) retryFunc {
	return func(_ context.Context) error {
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
	}
}

// ScheduledEventsAfterFunc returns a function that returns an error unless a kube-system 'Scheduled' event has occurred after the given time
// The  returned function is intended to be used with pkg/retry.
func ScheduledEventsAfterFunc(h *cluster.Host, since time.Time) retryFunc {
	return func(_ context.Context) error {
		output, err := h.ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "-n kube-system get events --field-selector reason=Scheduled -o json"), exec.HideOutput(), exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("failed to get kube system events: %w", err)
		}
		events := &statusEvents{}
		if err := json.Unmarshal([]byte(output), &events); err != nil {
			return fmt.Errorf("failed to decode kubectl output for kube-system events: %w", err)
		}
		for _, e := range events.Items {
			if e.EventTime.Before(since) {
				log.Tracef("%s: skipping prior event for %s: %s < %s", h, e.InvolvedObject.Name, e.EventTime.Format(time.RFC3339), since.Format(time.RFC3339))
				continue
			}
			log.Debugf("%s: found a 'Scheduled' event occuring after %s", h, since)
			return nil
		}
		return fmt.Errorf("didn't find any 'Scheduled' kube-system events after %s", since)
	}
}

// HTTPStatus returns a function that returns an error unless the expected status code is returned for a HTTP get to the url
func HTTPStatusFunc(h *cluster.Host, url string, expected ...int) retryFunc {
	return func(_ context.Context) error {
		return h.CheckHTTPStatus(url, expected...)
	}
}

// ServiceRunningFunc returns a function that returns an error until the service is running on the host
func ServiceRunningFunc(h *cluster.Host, service string) retryFunc {
	return func(_ context.Context) error {
		if !h.Configurer.ServiceIsRunning(h, service) {
			return fmt.Errorf("service %s is not running", service)
		}
		return nil
	}
}

// ServiceStoppedFunc returns a function that returns an error if the service is not running on the host
func ServiceStoppedFunc(h *cluster.Host, service string) retryFunc {
	return func(_ context.Context) error {
		if h.Configurer.ServiceIsRunning(h, service) {
			return fmt.Errorf("service %s is still running", service)
		}
		return nil
	}
}

// KubeAPIReadyFunc returns a function that returns an error unless the host's local kube api responds to /version
func KubeAPIReadyFunc(h *cluster.Host, config *v1beta1.Cluster) retryFunc {
	// If the anon-auth is disabled on kube api the version endpoint will give 401
	// thus we need to accept both 200 and 401 as valid statuses when checking kube api
	return HTTPStatusFunc(h, fmt.Sprintf("%s/version", config.Spec.NodeInternalKubeAPIURL(h)), 200, 401)
}
