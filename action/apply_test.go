package action

import (
	"testing"

	"github.com/k0sproject/k0sctl/phase"
	"github.com/stretchr/testify/require"
)

func TestApplyIncludesAirgapBeforeWorkerPhases(t *testing.T) {
	apply := NewApply(ApplyOptions{})
	airgapPhase := (&phase.AirgapBundles{}).Title()
	initializeK0s := (&phase.InitializeK0s{}).Title()
	installControllers := (&phase.InstallControllers{}).Title()
	installWorkers := (&phase.InstallWorkers{}).Title()
	upgradeWorkers := (&phase.UpgradeWorkers{}).Title()

	airgapIndex := apply.Phases.Index(airgapPhase)
	initializeIndex := apply.Phases.Index(initializeK0s)
	installControllersIndex := apply.Phases.Index(installControllers)
	installWorkersIndex := apply.Phases.Index(installWorkers)
	upgradeWorkersIndex := apply.Phases.Index(upgradeWorkers)

	require.NotEqual(t, -1, airgapIndex)
	require.NotEqual(t, -1, initializeIndex)
	require.NotEqual(t, -1, installControllersIndex)
	require.NotEqual(t, -1, installWorkersIndex)
	require.NotEqual(t, -1, upgradeWorkersIndex)
	require.Less(t, airgapIndex, initializeIndex)
	require.Less(t, airgapIndex, installControllersIndex)
	require.Less(t, airgapIndex, installWorkersIndex)
	require.Less(t, airgapIndex, upgradeWorkersIndex)
}
