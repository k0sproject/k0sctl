package action

import (
	"fmt"
	"os"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	log "github.com/sirupsen/logrus"
)

type Backup struct {
	Config            *v1beta1.Cluster
	Concurrency       int
	ConcurrentUploads int
	LogFile           *os.File
}

func (b Backup) Run() error {
	start := time.Now()

	manager := phase.Manager{Config: b.Config, Concurrency: b.Concurrency}
	lockPhase := &phase.Lock{}

	manager.AddPhase(
		&phase.Connect{},
		&phase.DetectOS{},
		lockPhase,
		&phase.PrepareHosts{},
		&phase.GatherFacts{},
		&phase.GatherK0sFacts{},
		&phase.RunHooks{Stage: "before", Action: "backup"},
		&phase.Backup{},
		&phase.RunHooks{Stage: "after", Action: "backup"},
		&phase.Unlock{Cancel: lockPhase.Cancel},
		&phase.Disconnect{},
	)

	analytics.Client.Publish("backup-start", map[string]interface{}{})

	if err := manager.Run(); err != nil {
		analytics.Client.Publish("backup-failure", map[string]interface{}{"clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
		if b.LogFile != nil {
			log.Errorf("backup failed - log file saved to %s", b.LogFile.Name())
		}
		return err
	}

	analytics.Client.Publish("backup-success", map[string]interface{}{"duration": time.Since(start), "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})

	duration := time.Since(start).Truncate(time.Second)
	text := fmt.Sprintf("==> Finished in %s", duration)
	log.Infof(phase.Colorize.Green(text).String())
	return nil
}
