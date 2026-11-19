package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/rig/v2/protocol"
	"github.com/k0sproject/version"

	log "github.com/sirupsen/logrus"
)

// this file contains functions that return functions that can be used with pkg/retry to wait on certain
// status conditions of nodes

type retryFunc func(context.Context) error

// kubeNodeStatus represents the output of `kubectl get node <name> -o json` for a single Node
type kubeNodeStatus struct {
	Status struct {
		Conditions []struct {
			Status string `json:"status"`
			Type   string `json:"type"`
		} `json:"conditions"`
		NodeInfo struct {
			KubeletVersion string `json:"kubeletVersion"`
		} `json:"nodeInfo"`
	} `json:"status"`
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

// KubeNodeReadyFunc returns a function that returns an error unless the node is ready according to "kubectl get node".
// A dropped connection needs no special handling here: rig reconnects transparently on the next attempt, so a transient
// connection loss simply retries. Only a genuinely non-retryable failure (auth, host key mismatch, ...) aborts the loop.
func KubeNodeReadyFunc(h *cluster.Host) retryFunc {
	return func(_ context.Context) error {
		_, err := kubeNodeReady(h)
		return err
	}
}

// KubeNodeVersionReadyFunc returns a function that returns an error unless the node is ready and its kubelet reports
// the version v. Right after a restart, the node can still be ready on the status the previous kubelet reported, so
// the version tells whether the restarted kubelet has registered.
//
// The kubelet does not report the k0s version (v1.36.5+k0s.1) but its Kubernetes build version (v1.36.5+k0s), and
// the pre-release part of a k0s version (v1.36.5-rc.0+k0s.0, or v1.36.5-dev+k0s of a development build) is not
// necessarily a part of it. Only the numeric major.minor.patch parts are compared, so an upgrade that changes
// nothing else can't be told apart this way.
func KubeNodeVersionReadyFunc(h *cluster.Host, v *version.Version) retryFunc {
	return func(_ context.Context) error {
		status, err := kubeNodeReady(h)
		if err != nil {
			return err
		}
		kubeletVersion := status.Status.NodeInfo.KubeletVersion
		if !sameNumericVersion(kubeletVersion, v) {
			return fmt.Errorf("node %s kubelet version is %q, waiting for %s", h.KubernetesNodeName(), kubeletVersion, numericVersion(v))
		}
		return nil
	}
}

// numericVersion returns the major.minor.patch part of v, such as v1.36.5.
func numericVersion(v *version.Version) string {
	s := v.Segments()
	return fmt.Sprintf("v%d.%d.%d", s[0], s[1], s[2])
}

// sameNumericVersion returns true when the kubelet version has the same major.minor.patch as v.
func sameNumericVersion(kubeletVersion string, v *version.Version) bool {
	kv, err := version.NewVersion(kubeletVersion)
	if err != nil {
		return false
	}
	return numericVersion(kv) == numericVersion(v)
}

func kubeNodeReady(h *cluster.Host) (*kubeNodeStatus, error) {
	nodeName := h.KubernetesNodeName()
	output, err := h.Sudo().ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get node %s -o json", h.FS().ShellQuote(nodeName)), cmd.HideOutput())
	if err != nil {
		if errors.Is(err, protocol.ErrNonRetryable) {
			return nil, errors.Join(retry.ErrAbort, fmt.Errorf("failed to get node status: %w", err))
		}
		return nil, fmt.Errorf("failed to get node status: %w", err)
	}
	status := &kubeNodeStatus{}
	if err := json.Unmarshal([]byte(output), status); err != nil {
		return nil, fmt.Errorf("failed to decode kubectl get node status output: %w", err)
	}
	for _, c := range status.Status.Conditions {
		if c.Type == "Ready" {
			if c.Status == "True" {
				return status, nil
			}
			return nil, fmt.Errorf("node %s is not ready", nodeName)
		}
	}
	return nil, fmt.Errorf("node %s 'Ready' condition not found", nodeName)
}

// K0sPID returns the process ID of the k0s instance running on the host, as
// reported by "k0s status". It fails when k0s is not running, for example while
// the service manager waits to restart a k0s that exited.
func K0sPID(ctx context.Context, h *cluster.Host) (int, error) {
	output, err := h.Sudo().ExecOutputContext(ctx, h.Configurer.K0sCmdf("status -o json"), cmd.HideOutput())
	if err != nil {
		return 0, fmt.Errorf("k0s is not running: %w", err)
	}
	status := struct {
		Pid int `json:"Pid"`
	}{}
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return 0, fmt.Errorf("failed to decode k0s status output: %w", err)
	}
	if status.Pid == 0 {
		return 0, errors.New("k0s status did not report a process id")
	}
	return status.Pid, nil
}

// K0sDynamicConfigReadyFunc returns a function that returns an error unless the k0s dynamic config has been reconciled
func K0sDynamicConfigReadyFunc(h *cluster.Host) retryFunc {
	return func(_ context.Context) error {
		output, err := h.Sudo().ExecOutput(h.Configurer.K0sCmdf("kubectl --data-dir=%s -n kube-system get event --field-selector involvedObject.name=k0s -o json", h.K0sDataDir()))
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
		output, err := h.Sudo().ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "-n kube-system get events --field-selector reason=Scheduled -o json"), cmd.HideOutput())
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
	return func(ctx context.Context) error {
		return h.CheckHTTPStatus(ctx, url, expected...)
	}
}

// ServiceRunningFunc returns a function that returns an error until the service is running on the host
func ServiceRunningFunc(h *cluster.Host, service string) retryFunc {
	return func(ctx context.Context) error {
		svc, err := h.Sudo().Service(service)
		if err != nil {
			return fmt.Errorf("get service %s: %w", service, err)
		}
		if !svc.IsRunning(ctx) {
			return fmt.Errorf("service %s is not running", service)
		}
		return nil
	}
}

// ServiceStoppedFunc returns a function that returns an error if the service is not running on the host
func ServiceStoppedFunc(h *cluster.Host, service string) retryFunc {
	return func(ctx context.Context) error {
		svc, err := h.Sudo().Service(service)
		if err != nil {
			return fmt.Errorf("get service %s: %w", service, err)
		}
		if svc.IsRunning(ctx) {
			return fmt.Errorf("service %s is still running", service)
		}
		return nil
	}
}
