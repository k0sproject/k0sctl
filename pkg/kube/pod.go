package kube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"al.essio.dev/pkg/shellescape"
	corev1 "k8s.io/api/core/v1"
)

// PodFilter is a search filter for listing pods.
type PodFilter struct {
	Namespace      string   // if empty, use --all-namespaces
	NodeName       string   // for spec.nodeName
	LabelSelectors []string // ["key=value", "key!=value"] etc.
	FieldSelectors []string // ["status.phase=Running", "spec.nodeName=node1"]
	OwnerKind      string   // e.g. "DaemonSet", "Deployment", etc.
}

func (f *PodFilter) ToKubectlArgs() string {
	args := []string{"get", "pods", "-o", "json"}

	if f.Namespace != "" {
		args = append(args, "-n", shellescape.Quote(f.Namespace))
	} else {
		args = append(args, "--all-namespaces")
	}

	if len(f.LabelSelectors) > 0 {
		labels := strings.Join(f.LabelSelectors, ",")
		args = append(args, "-l", shellescape.Quote(labels))
	}

	fieldSelectors := append([]string{}, f.FieldSelectors...)
	if f.NodeName != "" {
		fieldSelectors = append(fieldSelectors, "spec.nodeName="+f.NodeName)
	}
	if len(fieldSelectors) > 0 {
		fields := strings.Join(fieldSelectors, ",")
		args = append(args, "--field-selector", shellescape.Quote(fields))
	}

	return strings.Join(args, " ")
}

// PodInfo contains information about a pod.
type PodInfo struct {
	Namespace         string
	Name              string
	NodeName          string
	OwnerKind         string
	OwnerName         string
	Status            string
	PriorityClassName string
}

// PodInfoParser is a io.WriteCloser that parses kubectl output on Close().
type PodInfoParser struct {
	buf    bytes.Buffer
	closed bool
	pods   []*PodInfo
}

// Write data to the buffer.
func (p *PodInfoParser) Write(data []byte) (int, error) {
	if p.closed {
		return 0, errors.New("write after close")
	}
	return p.buf.Write(data)
}

// Close the buffer and parse the JSON data.
func (p *PodInfoParser) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true

	var podList corev1.PodList
	if err := json.Unmarshal(p.buf.Bytes(), &podList); err != nil {
		return fmt.Errorf("failed to decode pod list: %w", err)
	}

	for _, item := range podList.Items {
		pod := &PodInfo{
			Name:              item.Name,
			Namespace:         item.Namespace,
			NodeName:          item.Spec.NodeName,
			Status:            string(item.Status.Phase),
			PriorityClassName: item.Spec.PriorityClassName,
		}
		if len(item.OwnerReferences) > 0 {
			pod.OwnerKind = item.OwnerReferences[0].Kind
			pod.OwnerName = item.OwnerReferences[0].Name
		}
		p.pods = append(p.pods, pod)
	}

	p.buf.Reset()

	return nil
}

// Pods returns the parsed pod list.
func (p *PodInfoParser) Pods() []*PodInfo {
	return p.pods
}

// NewPodInfoParser creates a new PodInfoParser.
func NewPodInfoParser() *PodInfoParser {
	return &PodInfoParser{}
}
