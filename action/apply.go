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

type Apply struct {
	// Config is the k0sctl config
	Config *v1beta1.Cluster
	// Concurrency is the number of concurrent actions to run
	Concurrency int
	// ConcurrentUploads is the number of concurrent uploads to run
	ConcurrentUploads int
	// DisableDowngradeCheck skips the downgrade check
	DisableDowngradeCheck bool
	// Force allows forced installation in case of certain failures
	Force bool
	// NoWait skips waiting for the cluster to be ready
	NoWait bool
	// NoDrain skips draining worker nodes
	NoDrain bool
	// RestoreFrom is the path to a cluster backup archive to restore the state from
	RestoreFrom string
	// KubeconfigOut is the path to write the kubeconfig to
	KubeconfigOut string
	// KubeconfigAPIAddress is the API address to use in the kubeconfig
	KubeconfigAPIAddress string
	// LogFile is the path where log will be found from
	LogFile *os.File
}

func (a Apply) Run() error {
	start := time.Now()

	if a.Config == nil {
		return fmt.Errorf("config is nil")
	}

	phase.NoWait = a.NoWait
	phase.Force = a.Force

	manager := phase.Manager{Config: a.Config, Concurrency: a.Concurrency, ConcurrentUploads: a.ConcurrentUploads}
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
		&phase.ValidateFacts{SkipDowngradeCheck: a.DisableDowngradeCheck},
		&phase.UploadBinaries{},
		&phase.DownloadK0s{},
		&phase.InstallBinaries{},
		&phase.RunHooks{Stage: "before", Action: "apply"},
		&phase.PrepareArm{},
		&phase.ConfigureK0s{},
		&phase.Restore{
			RestoreFrom: a.RestoreFrom,
		},
		&phase.InitializeK0s{},
		&phase.InstallControllers{},
		&phase.InstallWorkers{},
		&phase.UpgradeControllers{},
		&phase.UpgradeWorkers{NoDrain: a.NoDrain},
		&phase.ResetWorkers{NoDrain: a.NoDrain},
		&phase.ResetControllers{NoDrain: a.NoDrain},
		&phase.RunHooks{Stage: "after", Action: "apply"},
	)

	var kubeCfgPhase *phase.GetKubeconfig
	if a.KubeconfigOut != "" {
		kubeCfgPhase = &phase.GetKubeconfig{APIAddress: a.KubeconfigAPIAddress}
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
		if lf := a.LogFile; lf != nil {
			log.Errorf("apply failed - log file saved to %s", lf.Name())
		}
		return result
	}

	analytics.Client.Publish("apply-success", map[string]interface{}{"duration": time.Since(start), "clusterID": manager.Config.Spec.K0s.Metadata.ClusterID})
	if a.KubeconfigOut != "" {
		if err := os.WriteFile(a.KubeconfigOut, []byte(manager.Config.Metadata.Kubeconfig), 0644); err != nil {
			log.Warnf("failed to write kubeconfig to %s: %v", a.KubeconfigOut, err)
		} else {
			log.Infof("kubeconfig written to %s", a.KubeconfigOut)
		}
	}

	duration := time.Since(start).Truncate(time.Second)
	text := fmt.Sprintf("==> Finished in %s", duration)
	log.Infof(phase.Colorize.Green(text).String())

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

	if a.KubeconfigOut != "" {
		log.Infof("Tip: To access the cluster you can now fetch the admin kubeconfig using:")
		log.Infof("     " + phase.Colorize.Cyan("k0sctl kubeconfig").String())
	}

	return nil
}
