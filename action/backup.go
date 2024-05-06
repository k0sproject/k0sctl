package action

import (
	"fmt"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	log "github.com/sirupsen/logrus"
)

type Backup struct {
	// Manager is the phase manager
	Manager *phase.Manager
}

func (b Backup) Run() error {
	start := time.Now()

	lockPhase := &phase.Lock{}

	b.Manager.AddPhase(
		&phase.Connect{},
		&phase.DetectOS{},
		lockPhase,
		&phase.PrepareHosts{},
		&phase.GatherFacts{SkipMachineIDs: true},
		&phase.GatherK0sFacts{},
		&phase.RunHooks{Stage: "before", Action: "backup"},
		&phase.Backup{},
		&phase.RunHooks{Stage: "after", Action: "backup"},
		&phase.Unlock{Cancel: lockPhase.Cancel},
		&phase.Disconnect{},
	)

	analytics.Client.Publish("backup-start", map[string]interface{}{})

	if err := b.Manager.Run(); err != nil {
		analytics.Client.Publish("backup-failure", map[string]interface{}{"clusterID": b.Manager.Config.Spec.K0s.Metadata.ClusterID})
		return err
	}

	analytics.Client.Publish("backup-success", map[string]interface{}{"duration": time.Since(start), "clusterID": b.Manager.Config.Spec.K0s.Metadata.ClusterID})

	duration := time.Since(start).Truncate(time.Second)
	text := fmt.Sprintf("==> Finished in %s", duration)
	log.Infof(phase.Colorize.Green(text).String())
	return nil
}
