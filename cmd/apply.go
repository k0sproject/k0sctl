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

var applyCommand = &cli.Command{
	Name:  "apply",
	Usage: "Apply a k0sctl configuration",
	Flags: []cli.Flag{
		configFlag,
		&cli.BoolFlag{
			Name:  "no-wait",
			Usage: "Do not wait for worker nodes to join",
		},
		&cli.BoolFlag{
			Name:  "no-drain",
			Usage: "Do not drain worker nodes when upgrading",
		},
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

		c := config.Cluster{}
		if err := yaml.UnmarshalStrict([]byte(content), &c); err != nil {
			return err
		}

		if err := c.Validate(); err != nil {
			return err
		}

		phase.NoWait = ctx.Bool("no-wait")

		manager := phase.Manager{Config: &c}

		manager.AddPhase(
			&phase.Connect{},
			&phase.DetectOS{},
			&phase.PrepareHosts{},
			&phase.GatherFacts{},
			&phase.DownloadBinaries{},
			&phase.UploadBinaries{},
			&phase.DownloadK0s{},
			&phase.UploadFiles{},
			&phase.ValidateHosts{},
			&phase.GatherK0sFacts{},
			&phase.ValidateFacts{},
			&phase.ConfigureK0s{},
			&phase.InitializeK0s{},
			&phase.InstallControllers{},
			&phase.InstallWorkers{},
			&phase.UpgradeControllers{},
			&phase.UpgradeWorkers{
				NoDrain: ctx.Bool("no-drain"),
			},
			&phase.Disconnect{},
		)

		if err := analytics.Client.Publish("apply-start", map[string]interface{}{}); err != nil {
			return err
		}

		if err := manager.Run(); err != nil {
			_ = analytics.Client.Publish("apply-failure", map[string]interface{}{"clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
			if lf, err := LogFile(); err == nil {
				if ln, ok := lf.(interface{ Name() string }); ok {
					log.Errorf("apply failed - log file saved to %s", ln.Name())
				}
			}
			return err
		}

		_ = analytics.Client.Publish("apply-success", map[string]interface{}{"duration": time.Since(start), "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})

		duration := time.Since(start).Truncate(time.Second)
		text := fmt.Sprintf("==> Finished in %s", duration)
		log.Infof(aurora.Green(text).String())

		log.Infof("k0s cluster version %s is now installed", c.Spec.K0s.Version)
		log.Infof("Tip: To access the cluster you can now fetch the admin kubeconfig using:")
		log.Infof("     " + aurora.Cyan("k0sctl kubeconfig").String())

		return nil
	},
}
