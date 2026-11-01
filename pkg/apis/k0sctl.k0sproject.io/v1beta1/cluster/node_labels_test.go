package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestNodeLabels_Unmarshal(t *testing.T) {
	h := Host{}
	require.NoError(t, yaml.Unmarshal([]byte(`
role: controller+worker
labels:
  apply:
    - node-role.kubernetes.io/control-plane=
    - example.com/zone=a
  delete:
    - node-role.kubernetes.io/master
taints:
  apply:
    - dedicated=gpu:NoSchedule
  delete:
    - node-role.kubernetes.io/master
`), &h))
	require.Equal(t, []string{"node-role.kubernetes.io/control-plane=", "example.com/zone=a"}, h.NodeLabels.Apply)
	require.Equal(t, []string{"node-role.kubernetes.io/master"}, h.NodeLabels.Delete)
	require.Equal(t, []string{"dedicated=gpu:NoSchedule"}, h.NodeTaints.Apply)
	require.Equal(t, []string{"node-role.kubernetes.io/master"}, h.NodeTaints.Delete)
	require.NoError(t, h.Validate())
}

func TestNodeLabels_Validate(t *testing.T) {
	require.NoError(t, NodeLabels{
		Apply:  []string{"node-role.kubernetes.io/control-plane=", "zone=a"},
		Delete: []string{"node-role.kubernetes.io/master"},
	}.Validate())

	require.ErrorContains(t, NodeLabels{Apply: []string{"zone"}}.Validate(), "key=value form")
	require.ErrorContains(t, NodeLabels{Apply: []string{"-zone=a"}}.Validate(), "invalid label key")
	require.ErrorContains(t, NodeLabels{Apply: []string{"zone=a b"}}.Validate(), "invalid value")
	require.ErrorContains(t, NodeLabels{Delete: []string{"bad key"}}.Validate(), "invalid label key")
	require.ErrorContains(t, NodeLabels{Apply: []string{"zone=a"}, Delete: []string{"zone"}}.Validate(), "both applied and deleted")
}

func TestNodeLabels_Changes(t *testing.T) {
	labels := NodeLabels{
		Apply:  []string{"zone=a", "tier=web", "node-role.kubernetes.io/control-plane="},
		Delete: []string{"node-role.kubernetes.io/master", "absent"},
	}
	current := map[string]string{
		"zone":                           "a",
		"tier":                           "db",
		"node-role.kubernetes.io/master": "",
	}

	apply, del := labels.Changes(current)
	require.Equal(t, []string{"tier=web", "node-role.kubernetes.io/control-plane="}, apply)
	require.Equal(t, []string{"node-role.kubernetes.io/master"}, del)

	apply, del = labels.Changes(map[string]string{
		"zone":                                  "a",
		"tier":                                  "web",
		"node-role.kubernetes.io/control-plane": "",
	})
	require.Empty(t, apply)
	require.Empty(t, del)
}

func TestNodeTaints_Validate(t *testing.T) {
	require.NoError(t, NodeTaints{
		Apply:  []string{"dedicated=gpu:NoSchedule", "maintenance:NoExecute"},
		Delete: []string{"node-role.kubernetes.io/master", "other:PreferNoSchedule"},
	}.Validate())

	require.ErrorContains(t, NodeTaints{Apply: []string{"dedicated=gpu"}}.Validate(), "key=value:Effect")
	require.ErrorContains(t, NodeTaints{Apply: []string{"dedicated=gpu:Never"}}.Validate(), "effect must be one of")
	require.ErrorContains(t, NodeTaints{Delete: []string{"dedicated=gpu:NoSchedule"}}.Validate(), "key or key:Effect form")
	require.ErrorContains(t, NodeTaints{Delete: []string{"dedicated:Never"}}.Validate(), "effect must be one of")
	require.ErrorContains(t, NodeTaints{
		Apply:  []string{"dedicated=gpu:NoSchedule"},
		Delete: []string{"dedicated"},
	}.Validate(), "both applied and deleted")
	require.NoError(t, NodeTaints{
		Apply:  []string{"dedicated=gpu:NoSchedule"},
		Delete: []string{"dedicated:NoExecute"},
	}.Validate())
}

func TestNodeTaints_Changes(t *testing.T) {
	taints := NodeTaints{
		Apply:  []string{"dedicated=gpu:NoSchedule", "maintenance:NoExecute", "zone=a:PreferNoSchedule"},
		Delete: []string{"node-role.kubernetes.io/master", "other:NoExecute", "absent"},
	}
	// The format of Host.Taints: key=value:Effect, with an empty value when the taint has none.
	current := []string{
		"dedicated=gpu:NoSchedule",
		"zone=b:PreferNoSchedule",
		"node-role.kubernetes.io/master=:NoSchedule",
		"other=:NoSchedule",
	}

	apply, del := taints.Changes(current)
	require.Equal(t, []string{"maintenance:NoExecute", "zone=a:PreferNoSchedule"}, apply)
	require.Equal(t, []string{"node-role.kubernetes.io/master"}, del)

	apply, del = taints.Changes([]string{""})
	require.Equal(t, taints.Apply, apply)
	require.Empty(t, del)
}

func TestValidation_NodeLabelsAndTaints(t *testing.T) {
	h := Host{Role: "controller", NodeLabels: NodeLabels{Apply: []string{"zone=a"}}}
	require.ErrorContains(t, h.Validate(), "labels can not be set for the controller role")

	h = Host{Role: "controller", NodeTaints: NodeTaints{Apply: []string{"dedicated=gpu:NoSchedule"}}}
	require.ErrorContains(t, h.Validate(), "taints can not be set for the controller role")

	h = Host{Role: "worker", NodeLabels: NodeLabels{Apply: []string{"zone"}}}
	require.ErrorContains(t, h.Validate(), "key=value form")

	h = Host{Role: "worker", NodeTaints: NodeTaints{Apply: []string{"dedicated=gpu"}}}
	require.ErrorContains(t, h.Validate(), "key=value:Effect")

	h = Host{
		Role:       "controller+worker",
		NodeLabels: NodeLabels{Apply: []string{"node-role.kubernetes.io/control-plane="}},
		NodeTaints: NodeTaints{Delete: []string{"node-role.kubernetes.io/master"}},
	}
	require.NoError(t, h.Validate())
}
