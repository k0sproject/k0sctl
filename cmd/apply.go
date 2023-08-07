package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	log "github.com/sirupsen/logrus"

	"github.com/urfave/cli/v2"
)

var applyCommand = &cli.Command{
	Name:  "apply",
	Usage: "Apply a k0sctl configuration",
	Flags: []cli.Flag{
		configFlag,
		concurrencyFlag,
		concurrentUploadsFlag,
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
		&cli.StringFlag{
			Name:      "kubeconfig-out",
			Usage:     "Write kubeconfig to given path after a successful apply",
			TakesFile: true,
		},
		&cli.StringFlag{
			Name:  "kubeconfig-api-address",
			Usage: "Override the API address in the kubeconfig when kubeconfig-out is set",
		},
		&cli.BoolFlag{
			Name:   "disable-downgrade-check",
			Usage:  "Skip downgrade check",
			Hidden: true,
		},
		&cli.BoolFlag{
			Name:  "force",
			Usage: "Attempt a forced installation in case of certain failures",
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
		phase.NoWait = ctx.Bool("no-wait")
		phase.Force = ctx.Bool("force")

		manager := phase.Manager{Config: ctx.Context.Value(ctxConfigKey{}).(*v1beta1.Cluster), Concurrency: ctx.Int("concurrency"), ConcurrentUploads: ctx.Int("concurrent-uploads")}
		lockPhase := &phase.Lock{}

		manager.AddPhase(
			&phase.Connect{},
			&phase.DetectOS{},
			lockPhase,
			&phase.PrepareHosts{},
			&phase.GatherFacts{},
			&phase.DownloadBinaries{},
			&phase.UploadFiles{},
			&phase.ValidateHosts{},
			&phase.GatherK0sFacts{},
			&phase.ValidateFacts{SkipDowngradeCheck: ctx.Bool("disable-downgrade-check")},
			&phase.UploadBinaries{},
			&phase.DownloadK0s{},
			&phase.InstallBinaries{},
			&phase.RunHooks{Stage: "before", Action: "apply"},
			&phase.PrepareArm{},
			&phase.ConfigureK0s{},
			&phase.Restore{
				RestoreFrom: ctx.String("restore-from"),
			},
			&phase.InitializeK0s{},
			&phase.InstallControllers{},
			&phase.InstallWorkers{},
			&phase.UpgradeControllers{},
			&phase.UpgradeWorkers{
				NoDrain: ctx.Bool("no-drain"),
			},
			&phase.ResetWorkers{
				NoDrain: ctx.Bool("no-drain"),
			},
			&phase.ResetControllers{
				NoDrain: ctx.Bool("no-drain"),
			},
			&phase.RunHooks{Stage: "after", Action: "apply"},
		)

		kubecfgOut := ctx.String("kubeconfig-out")
		var kubeCfgPhase *phase.GetKubeconfig
		if kubecfgOut != "" {
			kubeCfgPhase = &phase.GetKubeconfig{APIAddress: ctx.String("kubeconfig-api-address")}
			manager.AddPhase(kubeCfgPhase)
		}

		manager.AddPhase(
			&phase.Unlock{Cancel: lockPhase.Cancel},
			&phase.Disconnect{},
		)

		analytics.Client.Publish("apply-start", map[string]interface{}{})

		var result error

		if result = manager.Run(); result != nil {
			analytics.Client.Publish("apply-failure", map[string]interface{}{"clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
			if lf, err := LogFile(); err == nil {
				if ln, ok := lf.(interface{ Name() string }); ok {
					log.Errorf("apply failed - log file saved to %s", ln.Name())
				}
			}
			return result
		}

		analytics.Client.Publish("apply-success", map[string]interface{}{"duration": time.Since(start), "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
		if kubecfgOut != "" {
			if err := os.WriteFile(kubecfgOut, []byte(manager.Config.Metadata.Kubeconfig), 0644); err != nil {
				log.Warnf("failed to write kubeconfig to %s: %v", kubecfgOut, err)
			} else {
				log.Infof("kubeconfig written to %s", kubecfgOut)
			}
		}

		duration := time.Since(start).Truncate(time.Second)
		text := fmt.Sprintf("==> Finished in %s", duration)
		log.Infof(Colorize.Green(text).String())

		uninstalled := false
		for _, host := range manager.Config.Spec.Hosts {
			if host.Reset {
				uninstalled = true
			}
		}
		if uninstalled {
			log.Info("There were nodes that got uninstalled during the apply phase. Please remove them from your k0sctl config file")
		}

		log.Infof("k0s cluster version %s is now installed", manager.Config.Spec.K0s.Version)
		log.Infof("Tip: To access the cluster you can now fetch the admin kubeconfig using:")
		log.Infof("     " + Colorize.Cyan("k0sctl kubeconfig").String())

		return nil
	},
}
