package cluster

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

// taintEffects are the taint effects accepted by Kubernetes.
var taintEffects = []string{"NoSchedule", "PreferNoSchedule", "NoExecute"}

// NodeLabels lists the labels k0sctl manages on the host's Kubernetes node.
// Unlike labels given through installFlags, they are set through the
// Kubernetes API, so they can include labels that a kubelet is not allowed to
// set on itself, and they are checked again on every apply.
type NodeLabels struct {
	// Apply holds labels in key=value form to set on the node.
	Apply []string `yaml:"apply,omitempty"`
	// Delete holds the keys of labels to remove from the node.
	Delete []string `yaml:"delete,omitempty"`
}

// IsEmpty returns true when there are no labels to apply or delete.
func (l NodeLabels) IsEmpty() bool {
	return len(l.Apply) == 0 && len(l.Delete) == 0
}

// Validate checks the label syntax.
func (l NodeLabels) Validate() error {
	var errs []error
	applied := make(map[string]struct{}, len(l.Apply))
	for _, label := range l.Apply {
		key, _, err := parseNodeLabel(label)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		applied[key] = struct{}{}
	}
	for _, key := range l.Delete {
		if err := validateNodeLabelKey(key); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, ok := applied[key]; ok {
			errs = append(errs, fmt.Errorf("label %q is both applied and deleted", key))
		}
	}
	return errors.Join(errs...)
}

// Changes compares the labels with the current labels of the node and returns
// the labels that need to be set (key=value) and the keys that need to be removed.
func (l NodeLabels) Changes(current map[string]string) (apply []string, del []string) {
	for _, label := range l.Apply {
		key, value, err := parseNodeLabel(label)
		if err != nil {
			continue
		}
		if existing, ok := current[key]; !ok || existing != value {
			apply = append(apply, label)
		}
	}
	for _, key := range l.Delete {
		if _, ok := current[key]; ok {
			del = append(del, key)
		}
	}
	return apply, del
}

func parseNodeLabel(label string) (string, string, error) {
	key, value, ok := strings.Cut(label, "=")
	if !ok {
		return "", "", fmt.Errorf("invalid label %q: must be in key=value form", label)
	}
	if err := validateNodeLabelKey(key); err != nil {
		return "", "", err
	}
	if msgs := k8svalidation.IsValidLabelValue(value); len(msgs) > 0 {
		return "", "", fmt.Errorf("invalid value in label %q: %s", label, strings.Join(msgs, "; "))
	}
	return key, value, nil
}

func validateNodeLabelKey(key string) error {
	if msgs := k8svalidation.IsQualifiedName(key); len(msgs) > 0 {
		return fmt.Errorf("invalid label key %q: %s", key, strings.Join(msgs, "; "))
	}
	return nil
}

// NodeTaints lists the taints k0sctl manages on the host's Kubernetes node.
type NodeTaints struct {
	// Apply holds taints in key=value:Effect or key:Effect form to set on the node.
	Apply []string `yaml:"apply,omitempty"`
	// Delete holds taints to remove from the node, in key:Effect form, or
	// just the key to remove the taint for every effect.
	Delete []string `yaml:"delete,omitempty"`
}

// IsEmpty returns true when there are no taints to apply or delete.
func (t NodeTaints) IsEmpty() bool {
	return len(t.Apply) == 0 && len(t.Delete) == 0
}

// Validate checks the taint syntax.
func (t NodeTaints) Validate() error {
	var errs []error
	applied := make(map[string]struct{}, len(t.Apply))
	for _, s := range t.Apply {
		taint, err := parseNodeTaint(s)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		applied[taint.key+":"+taint.effect] = struct{}{}
	}
	for _, s := range t.Delete {
		taint, err := parseNodeTaintSelector(s)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for a := range applied {
			if taint.matches(a) {
				errs = append(errs, fmt.Errorf("taint %q is both applied and deleted", s))
				break
			}
		}
	}
	return errors.Join(errs...)
}

// Changes compares the taints with the current taints of the node and returns
// the taints that need to be set and the taints that need to be removed, in
// the form kubectl taint accepts them (without the trailing "-" for removals).
// Current taints are expected in the key=value:Effect form.
func (t NodeTaints) Changes(current []string) (apply []string, del []string) {
	existing := make(map[string]nodeTaint, len(current))
	for _, s := range current {
		taint, err := parseNodeTaint(s)
		if err != nil {
			continue
		}
		existing[taint.key+":"+taint.effect] = taint
	}

	for _, s := range t.Apply {
		taint, err := parseNodeTaint(s)
		if err != nil {
			continue
		}
		if e, ok := existing[taint.key+":"+taint.effect]; !ok || e.value != taint.value {
			apply = append(apply, s)
		}
	}

	for _, s := range t.Delete {
		selector, err := parseNodeTaintSelector(s)
		if err != nil {
			continue
		}
		for keyEffect := range existing {
			if selector.matches(keyEffect) {
				del = append(del, s)
				break
			}
		}
	}
	return apply, del
}

type nodeTaint struct {
	key    string
	value  string
	effect string
}

// matches reports whether a taint identified by key:Effect is selected. A
// selector without an effect selects the key with any effect.
func (t nodeTaint) matches(keyEffect string) bool {
	key, effect, _ := strings.Cut(keyEffect, ":")
	if key != t.key {
		return false
	}
	return t.effect == "" || t.effect == effect
}

// parseNodeTaint parses a taint in the key=value:Effect or key:Effect form.
func parseNodeTaint(s string) (nodeTaint, error) {
	keyValue, effect, ok := strings.Cut(s, ":")
	if !ok {
		return nodeTaint{}, fmt.Errorf("invalid taint %q: must be in key=value:Effect or key:Effect form", s)
	}
	if !slices.Contains(taintEffects, effect) {
		return nodeTaint{}, fmt.Errorf("invalid taint %q: effect must be one of %s", s, strings.Join(taintEffects, ", "))
	}
	key, value, _ := strings.Cut(keyValue, "=")
	if err := validateNodeLabelKey(key); err != nil {
		return nodeTaint{}, fmt.Errorf("invalid taint %q: %w", s, err)
	}
	if msgs := k8svalidation.IsValidLabelValue(value); len(msgs) > 0 {
		return nodeTaint{}, fmt.Errorf("invalid value in taint %q: %s", s, strings.Join(msgs, "; "))
	}
	return nodeTaint{key: key, value: value, effect: effect}, nil
}

// parseNodeTaintSelector parses a taint to delete in the key or key:Effect form.
func parseNodeTaintSelector(s string) (nodeTaint, error) {
	key, effect, hasEffect := strings.Cut(s, ":")
	if strings.Contains(key, "=") {
		return nodeTaint{}, fmt.Errorf("invalid taint to delete %q: must be in key or key:Effect form", s)
	}
	if err := validateNodeLabelKey(key); err != nil {
		return nodeTaint{}, fmt.Errorf("invalid taint to delete %q: %w", s, err)
	}
	if hasEffect && !slices.Contains(taintEffects, effect) {
		return nodeTaint{}, fmt.Errorf("invalid taint to delete %q: effect must be one of %s", s, strings.Join(taintEffects, ", "))
	}
	return nodeTaint{key: key, effect: effect}, nil
}
