package cmd

import (
	"fmt"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

var backupCommand = &cli.Command{
	Name:  "backup",
	Usage: "Take backup of existing clusters state",
	Flags: []cli.Flag{
		configFlag,
		debugFlag,
		traceFlag,
		analyticsFlag,
	},
	Before: actions(initLogging, initConfig, displayLogo, initAnalytics, displayCopyright),
	After: func(ctx *cli.Context) error {
		analytics.Client.Close()
		return nil
	},
	Action: func(ctx *cli.Context) error {
		start := time.Now()
		content := ctx.String("config")
		log.Debugf("Loaded configuration:\n%s", content)

		c := config.Cluster{}
		if err := yaml.UnmarshalStrict([]byte(content), &c); err != nil {
			return err
		}

		if err := c.Validate(); err != nil {
			return err
		}

		manager := phase.Manager{Config: &c}
		manager.AddPhase(
			&phase.Connect{},
			&phase.DetectOS{},
			&phase.GatherFacts{},
			&phase.GatherK0sFacts{},
			&phase.RunHooks{Stage: "before", Action: "backup"},
			&phase.Backup{},
			&phase.RunHooks{Stage: "after", Action: "backup"},
			&phase.Disconnect{},
		)

		if err := analytics.Client.Publish("backup-start", map[string]interface{}{}); err != nil {
			return err
		}

		if err := manager.Run(); err != nil {
			_ = analytics.Client.Publish("backup-failure", map[string]interface{}{"clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
			if lf, err := LogFile(); err == nil {
				if ln, ok := lf.(interface{ Name() string }); ok {
					log.Errorf("backup failed - log file saved to %s", ln.Name())
				}
			}
			return err
		}

		_ = analytics.Client.Publish("backup-success", map[string]interface{}{"duration": time.Since(start), "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})

		duration := time.Since(start).Truncate(time.Second)
		text := fmt.Sprintf("==> Finished in %s", duration)
		log.Infof(aurora.Green(text).String())
		return nil
	},
}
