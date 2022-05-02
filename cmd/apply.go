package cmd

import (
	"fmt"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	log "github.com/sirupsen/logrus"

	"github.com/urfave/cli/v2"
)

var ApplyPhases = []phase.Phase{
	&phase.Connect{},
	&phase.DetectOS{},
	&phase.PrepareHosts{},
	&phase.GatherFacts{},
	&phase.DownloadBinaries{},
	&phase.UploadFiles{},
	&phase.ValidateHosts{},
	&phase.GatherK0sFacts{},
	&phase.ValidateFacts{},
	&phase.UploadBinaries{},
	&phase.DownloadK0s{},
	&phase.RunHooks{Stage: "before", Action: "apply"},
	&phase.PrepareArm{},
	&phase.ConfigureK0s{},
	&phase.Restore{},
	&phase.InitializeK0s{},
	&phase.InstallControllers{},
	&phase.InstallWorkers{},
	&phase.UpgradeControllers{},
	&phase.UpgradeWorkers{},
	&phase.RunHooks{Stage: "after", Action: "apply"},
	&phase.Disconnect{},
}

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
		&cli.StringFlag{
			Name:      "restore-from",
			Usage:     "Path to cluster backup archive to restore the state from",
			TakesFile: true,
		},
		&cli.BoolFlag{
			Name:   "disable-downgrade-check",
			Usage:  "Skip downgrade check",
			Hidden: true,
		},
		debugFlag,
		traceFlag,
		redactFlag,
		analyticsFlag,
		upgradeCheckFlag,
	},
	Before: actions(initLogging, startCheckUpgrade, initConfig, displayLogo, initAnalytics, displayCopyright, warnOldCache),
	After:  actions(reportCheckUpgrade, closeAnalytics),
	Action: func(ctx *cli.Context) error {
		start := time.Now()

		manager := phase.NewManager(ctx.Context)

		manager.AddPhase(ApplyPhases...)

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
		log.Infof(Colorize.Green(text).String())

		log.Infof("k0s cluster version %s is now installed", manager.Config.Spec.K0s.Version)
		log.Infof("Tip: To access the cluster you can now fetch the admin kubeconfig using:")
		log.Infof("     " + Colorize.Cyan("k0sctl kubeconfig").String())

		return nil
	},
}
