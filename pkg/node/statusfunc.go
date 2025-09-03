package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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

type daemonSetInfo struct {
	Metadata struct {
		Name       string `json:"name"`
		Generation int64  `json:"generation"`
	} `json:"metadata"`
	Spec struct {
		Selector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Spec struct {
				Containers []struct {
					Name  string `json:"name"`
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ObservedGeneration     int64 `json:"observedGeneration"`
		DesiredNumberScheduled int32 `json:"desiredNumberScheduled"`
		UpdatedNumberScheduled int32 `json:"updatedNumberScheduled"`
		NumberAvailable        int32 `json:"numberAvailable"`
	} `json:"status"`
}

type podList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			NodeName   string `json:"nodeName"`
			Containers []struct {
				Name  string `json:"name"`
				Image string `json:"image"`
			} `json:"containers"`
		} `json:"spec"`
		Status struct {
			Phase             string `json:"phase"`
			ContainerStatuses []struct {
				Name    string `json:"name"`
				Ready   bool   `json:"ready"`
				Image   string `json:"image"`
				ImageID string `json:"imageID"`
			} `json:"containerStatuses"`
		} `json:"status"`
	} `json:"items"`
}

// DaemonSetRolledOutFunc returns a retryFunc that waits until the given DaemonSet has:
//  1. been observed by the controller (observedGeneration == generation)
//  2. updatedNumberScheduled == desiredNumberScheduled
//  3. numberAvailable == desiredNumberScheduled
//  4. all matched pods have the specified container Ready and matching the template image
//
// If skipIfMissing is true and the DaemonSet is NotFound, it returns nil
// (useful for proxyless setups where kube-proxy DS is intentionally absent).
// (cluster-wide DaemonSet rollout checker removed; use per-node variant below)

// Optional convenience: kube-proxy waiter (skip if DS missing, e.g., proxyless CNI)
// (cluster-wide kube-proxy waiter removed; use KubeProxyRolledOutOnNodeFunc)

// KubeProxyRolledOutOnNodeFunc is like KubeProxyRolledOutFunc, but only checks the
// kube-proxy pod on the given node. This avoids waiting for a cluster-wide
// rollout when used as a per-host guardrail.
// KubeProxyRolledOutFunc checks kube-proxy rollout on the given host.
// Executes the status query on the same host.
func KubeProxyRolledOutFunc(h *cluster.Host) retryFunc {
    return DaemonSetRolledOutOnHostFunc(h, "kube-system", "kube-proxy", "kube-proxy", strings.ToLower(h.Metadata.Hostname), true)
}

// KubeProxyRolledOutFuncFrom checks kube-proxy rollout for target host `t`
// while executing kubectl on the query host `q` (typically a controller).
func KubeProxyRolledOutFuncFrom(q, t *cluster.Host) retryFunc {
    return DaemonSetRolledOutOnHostFunc(q, "kube-system", "kube-proxy", "kube-proxy", strings.ToLower(t.Metadata.Hostname), true)
}

// KubeProxyRolledOutClusterFunc waits for kube-proxy DS to match the desired
// state across all scheduled nodes in the cluster. The query is executed on `q`.
func KubeProxyRolledOutClusterFunc(q *cluster.Host) retryFunc {
    return DaemonSetRolledOutClusterFunc(q, "kube-system", "kube-proxy", "kube-proxy", true)
}

func fetchDaemonSet(h *cluster.Host, ns, name string) (*daemonSetInfo, error) {
	out, err := h.ExecOutput(
		h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "-n %s get ds %s -o json", ns, name),
		exec.HideOutput(), exec.Sudo(h),
	)
	if err != nil {
		return nil, wrapKubectlNotFound(err)
	}
	var ds daemonSetInfo
	if uerr := json.Unmarshal([]byte(out), &ds); uerr != nil {
		return nil, fmt.Errorf("failed to decode DaemonSet %s/%s: %w", ns, name, uerr)
	}
	return &ds, nil
}

// (cluster-wide DS completion assertion removed; per-node variant performs minimal checks)

func desiredContainerImage(ds *daemonSetInfo, containerName string) (string, error) {
	containers := ds.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		return "", fmt.Errorf("DaemonSet has no containers in pod template")
	}
	if containerName == "" {
		return containers[0].Image, nil
	}
	for _, c := range containers {
		if c.Name == containerName {
			return c.Image, nil
		}
	}
	return "", fmt.Errorf("container %q not found in DaemonSet template", containerName)
}

// (cluster-wide pod listing removed; use listPodsForDaemonSetOnNode)

func listPodsForDaemonSetOnHost(h *cluster.Host, ns string, ds *daemonSetInfo, nodeName string) (*podList, error) {
    selector := buildLabelSelector(ds.Spec.Selector.MatchLabels)
    out, err := h.ExecOutput(
        h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "-n %s get pods -l %s --field-selector spec.nodeName=%s -o json", ns, selector, nodeName),
        exec.HideOutput(), exec.Sudo(h),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to list pods for selector %q on node %q in %s: %w", selector, nodeName, ns, err)
    }
    var pods podList
    if uerr := json.Unmarshal([]byte(out), &pods); uerr != nil {
        return nil, fmt.Errorf("failed to decode pods for selector %q on node %q: %w", selector, nodeName, uerr)
    }
    return &pods, nil
}

func listPodsForDaemonSet(h *cluster.Host, ns string, ds *daemonSetInfo) (*podList, error) {
    selector := buildLabelSelector(ds.Spec.Selector.MatchLabels)
    out, err := h.ExecOutput(
        h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "-n %s get pods -l %s -o json", ns, selector),
        exec.HideOutput(), exec.Sudo(h),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to list pods for selector %q in %s: %w", selector, ns, err)
    }
    var pods podList
    if uerr := json.Unmarshal([]byte(out), &pods); uerr != nil {
        return nil, fmt.Errorf("failed to decode pods for selector %q: %w", selector, uerr)
    }
    return &pods, nil
}

// DaemonSetRolledOutOnHostFunc returns a retryFunc that waits until the given
// DaemonSet has been observed and the pod matching the DS on the specified
// node is Running, Ready, and using the desired image from the DS template.
// If skipIfMissing is true and the DaemonSet is NotFound, it returns nil.
func DaemonSetRolledOutOnHostFunc(h *cluster.Host, namespace, dsName, containerName, nodeName string, skipIfMissing bool) retryFunc {
    return func(_ context.Context) error {
        ds, err := fetchDaemonSet(h, namespace, dsName)
        if err != nil {
            if skipIfMissing && isNotFoundErr(err) {
                log.Infof("%s: DaemonSet %s/%s not found; skipping as requested", h, namespace, dsName)
                return nil
            }
            return err
        }

        // Ensure controller has observed the latest DS and rollout is in progress/completed cluster-wide.
        // We do not require updated/available == desired cluster-wide here; we only care about this node.
        if ds.Status.ObservedGeneration != ds.Metadata.Generation {
            return fmt.Errorf("DaemonSet not yet observed: gen=%d obs=%d", ds.Metadata.Generation, ds.Status.ObservedGeneration)
        }

        // If DS is configured but schedules zero pods overall, there's nothing to wait for.
        if ds.Status.DesiredNumberScheduled == 0 {
            log.Infof("%s: %s/%s desiredNumberScheduled=0; nothing to roll out", h, namespace, dsName)
            return nil
        }

        desiredImg, err := desiredContainerImage(ds, containerName)
        if err != nil {
            return err
        }

        pods, err := listPodsForDaemonSetOnHost(h, namespace, ds, nodeName)
        if err != nil {
            return err
        }
        if len(pods.Items) == 0 {
            return fmt.Errorf("no pods found for DaemonSet %s/%s on node %s", namespace, dsName, nodeName)
        }

        notReady, mismatches := verifyPodsReadyAndImage(pods, containerName, desiredImg)
        if notReady > 0 {
            return fmt.Errorf("%d containers NotReady for DaemonSet %s/%s on node %s", notReady, namespace, dsName, nodeName)
        }
        if mismatches > 0 {
            return fmt.Errorf("%d pods running unexpected image for DaemonSet %s/%s on node %s", mismatches, namespace, dsName, nodeName)
        }

        log.Debugf("%s: %s/%s rolled out on node %s: image=%s", h, namespace, dsName, nodeName, desiredImg)
        return nil
    }
}

// DaemonSetRolledOutClusterFunc waits for the DS to be fully rolled out across
// the cluster according to controller status and pod readiness/image checks.
// If skipIfMissing is true and DS is NotFound, it returns nil.
func DaemonSetRolledOutClusterFunc(h *cluster.Host, namespace, dsName, containerName string, skipIfMissing bool) retryFunc {
    return func(_ context.Context) error {
        ds, err := fetchDaemonSet(h, namespace, dsName)
        if err != nil {
            if skipIfMissing && isNotFoundErr(err) {
                log.Infof("%s: DaemonSet %s/%s not found; skipping as requested", h, namespace, dsName)
                return nil
            }
            return err
        }

        // Controller must have observed current generation and report full rollout.
        if ds.Status.ObservedGeneration != ds.Metadata.Generation {
            return fmt.Errorf("DaemonSet not yet observed: gen=%d obs=%d", ds.Metadata.Generation, ds.Status.ObservedGeneration)
        }
        if ds.Status.DesiredNumberScheduled == 0 {
            log.Infof("%s: %s/%s desiredNumberScheduled=0; nothing to roll out", h, namespace, dsName)
            return nil
        }

        desiredImg, err := desiredContainerImage(ds, containerName)
        if err != nil {
            return err
        }

        pods, err := listPodsForDaemonSet(h, namespace, ds)
        if err != nil {
            return err
        }
        if int32(len(pods.Items)) != ds.Status.DesiredNumberScheduled {
            return fmt.Errorf("pod count mismatch for DS %s/%s: have=%d desired=%d", namespace, dsName, len(pods.Items), ds.Status.DesiredNumberScheduled)
        }

        notReady, mismatches := verifyPodsReadyAndImage(pods, containerName, desiredImg)
        if notReady > 0 {
            return fmt.Errorf("%d containers NotReady for DaemonSet %s/%s", notReady, namespace, dsName)
        }
        if mismatches > 0 {
            return fmt.Errorf("%d pods running unexpected image for DaemonSet %s/%s", mismatches, namespace, dsName)
        }

        if ds.Status.UpdatedNumberScheduled != ds.Status.DesiredNumberScheduled || ds.Status.NumberAvailable != ds.Status.DesiredNumberScheduled {
            return fmt.Errorf("DaemonSet not fully rolled out: updated=%d available=%d desired=%d",
                ds.Status.UpdatedNumberScheduled, ds.Status.NumberAvailable, ds.Status.DesiredNumberScheduled)
        }

        log.Debugf("%s: %s/%s rolled out cluster-wide: desired=%d updated=%d available=%d image=%s", h, namespace, dsName,
            ds.Status.DesiredNumberScheduled, ds.Status.UpdatedNumberScheduled, ds.Status.NumberAvailable, desiredImg)
        return nil
    }
}

func verifyPodsReadyAndImage(pods *podList, containerName, desiredImg string) (notReady, mismatches int) {
	for _, p := range pods.Items {
		if p.Status.Phase != "Running" {
			notReady++
			continue
		}
		var podImg, imageID string
		var hasContainer, ready bool

		for _, c := range p.Spec.Containers {
			if containerName == "" || c.Name == containerName {
				podImg = c.Image
				break
			}
		}
		for _, cs := range p.Status.ContainerStatuses {
			if containerName == "" || cs.Name == containerName {
				hasContainer = true
				ready = cs.Ready
				imageID = cs.ImageID
				break
			}
		}
		if !hasContainer || !ready {
			notReady++
			continue
		}
		if !matchImage(desiredImg, podImg, imageID) {
			mismatches++
		}
	}
	return
}

func buildLabelSelector(labels map[string]string) string {
	// Simple AND of matchLabels (k=v,k2=v2,...)
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	// Deterministic order not required by kubectl, but harmless as-is.
	return strings.Join(parts, ",")
}

func matchImage(dsImage, podImage, podImageID string) bool {
	// Exact tag match
	if dsImage != "" && dsImage == podImage {
		return true
	}
	// Digest pin match: DS template uses @sha256:..., ensure pod's ImageID has same digest.
	if at := strings.Index(dsImage, "@sha256:"); at != -1 {
		digest := dsImage[at+1:] // "sha256:..."
		return strings.Contains(podImageID, digest)
	}
	return false
}

func wrapKubectlNotFound(err error) error {
	if err == nil {
		return nil
	}
	// Typical stderr: 'Error from server (NotFound): daemonsets.apps "kube-proxy" not found'
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "notfound") || strings.Contains(low, "not found") {
		return &notFoundError{err}
	}
	return err
}

type notFoundError struct{ error }

func (e *notFoundError) Unwrap() error { return e.error }

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	var nf *notFoundError
	if errors.As(err, &nf) {
		return true
	}
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "notfound") || strings.Contains(low, "not found")
}
