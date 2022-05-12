package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var BackupPhases = []phase.Phase{
	&phase.Connect{},
	&phase.DetectOS{},
	&phase.GatherFacts{},
	&phase.GatherK0sFacts{},
	&phase.RunHooks{Stage: "before", Action: "backup"},
	&phase.Backup{},
	&phase.RunHooks{Stage: "after", Action: "backup"},
	&phase.Disconnect{},
}

var backupCommand = &cli.Command{
	Name:  "backup",
	Usage: "Take backup of existing clusters state",
	Flags: []cli.Flag{
		configFlag,
		debugFlag,
		traceFlag,
		redactFlag,
		analyticsFlag,
		upgradeCheckFlag,
	},
	Before: Actions(InitLogging, startCheckUpgrade, InitConfig, DisplayLogo, initAnalytics, displayCopyright),
	After:  Actions(reportCheckUpgrade, closeAnalytics),
	Action: func(ctx *cli.Context) error {
		manager := phase.NewManager(ctx.Context, BackupPhases...)

		if err := analytics.Client.Publish("backup-start", map[string]interface{}{}); err != nil {
			return err
		}

		res := manager.Run(ctx)
		if !res.Success() {
			_ = analytics.Client.Publish("backup-failure", map[string]interface{}{"clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
			if lf, err := LogFile(); err == nil {
				if ln, ok := lf.(interface{ Name() string }); ok {
					log.Errorf("backup failed - log file saved to %s", ln.Name())
				}
			}
			return res
		}

		_ = analytics.Client.Publish("backup-success", map[string]interface{}{"duration": res.Duration, "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})

		text := fmt.Sprintf("==> Finished in %s", res.Duration)
		log.Infof(Colorize.Green(text).String())
		return nil
	},
}
