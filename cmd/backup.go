package cmd

import (
	"fmt"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var backupCommand = &cli.Command{
	Name:  "backup",
	Usage: "Take backup of existing clusters state",
	Flags: []cli.Flag{
		configFlag,
		concurrencyFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		analyticsFlag,
		upgradeCheckFlag,
	},
	Before: actions(initLogging, startCheckUpgrade, initConfig, displayLogo, initAnalytics, displayCopyright),
	After:  actions(reportCheckUpgrade, closeAnalytics),
	Action: func(ctx *cli.Context) error {
		start := time.Now()

		manager := phase.Manager{Config: ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster), Concurrency: ctx.Int("concurrency")}
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
			if lf, err := LogFile(); err == nil {
				if ln, ok := lf.(interface{ Name() string }); ok {
					log.Errorf("backup failed - log file saved to %s", ln.Name())
				}
			}
			return err
		}

		analytics.Client.Publish("backup-success", map[string]interface{}{"duration": time.Since(start), "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})

		duration := time.Since(start).Truncate(time.Second)
		text := fmt.Sprintf("==> Finished in %s", duration)
		log.Infof(Colorize.Green(text).String())
		return nil
	},
}
