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

	require.Less(t, apply.Phases.Index(airgapPhase), apply.Phases.Index(initializeK0s))
	require.Less(t, apply.Phases.Index(airgapPhase), apply.Phases.Index(installControllers))
	require.Less(t, apply.Phases.Index(airgapPhase), apply.Phases.Index(installWorkers))
	require.Less(t, apply.Phases.Index(airgapPhase), apply.Phases.Index(upgradeWorkers))
}
