package action

import (
	"fmt"
	"io"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/phase"

	log "github.com/sirupsen/logrus"
)

type Apply struct {
	// Manager is the phase manager
	Manager *phase.Manager
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
	// KubeconfigOut is a writer to write the kubeconfig to
	KubeconfigOut io.Writer
	// KubeconfigAPIAddress is the API address to use in the kubeconfig
	KubeconfigAPIAddress string
}

func (a Apply) Run() error {
	start := time.Now()

	phase.NoWait = a.NoWait
	phase.Force = a.Force

	lockPhase := &phase.Lock{}

	a.Manager.AddPhase(
		&phase.DefaultK0sVersion{},
		&phase.Connect{},
		&phase.DetectOS{},
		lockPhase,
		&phase.PrepareHosts{},
		&phase.GatherFacts{},
		&phase.ValidateHosts{},
		&phase.GatherK0sFacts{},
		&phase.ValidateFacts{SkipDowngradeCheck: a.DisableDowngradeCheck},
		&phase.RunHooks{Stage: "before", Action: "apply"},
		&phase.DownloadBinaries{},
		&phase.UploadK0s{},
		&phase.DownloadK0s{},
		&phase.UploadFiles{},
		&phase.InstallBinaries{}, // need to think how to handle validation if this is faked on plan
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

	if a.KubeconfigOut != nil {
		a.Manager.AddPhase(&phase.GetKubeconfig{APIAddress: a.KubeconfigAPIAddress})
	}

	a.Manager.AddPhase(
		&phase.Unlock{Cancel: lockPhase.Cancel},
		&phase.Disconnect{},
	)

	analytics.Client.Publish("apply-start", map[string]interface{}{})

	var result error

	if result = a.Manager.Run(); result != nil {
		analytics.Client.Publish("apply-failure", map[string]interface{}{"clusterID": a.Manager.Config.Spec.K0s.Metadata.ClusterID})
		log.Info(phase.Colorize.Red("==> Apply failed").String())
		return result
	}

	analytics.Client.Publish("apply-success", map[string]interface{}{"duration": time.Since(start), "clusterID": a.Manager.Config.Spec.K0s.Metadata.ClusterID})
	if a.KubeconfigOut != nil {
		if _, err := a.KubeconfigOut.Write([]byte(a.Manager.Config.Metadata.Kubeconfig)); err != nil {
			log.Warnf("failed to write kubeconfig to %s: %v", a.KubeconfigOut, err)
		}
	}

	duration := time.Since(start).Truncate(time.Second)
	text := fmt.Sprintf("==> Finished in %s", duration)
	log.Infof(phase.Colorize.Green(text).String())

	uninstalled := false
	for _, host := range a.Manager.Config.Spec.Hosts {
		if host.Reset {
			uninstalled = true
		}
	}
	if uninstalled {
		log.Info("There were nodes that got uninstalled during the apply phase. Please remove them from your k0sctl config file")
	}

	log.Infof("k0s cluster version %s is now installed", a.Manager.Config.Spec.K0s.Version)

	if a.KubeconfigOut != nil {
		log.Infof("Tip: To access the cluster you can now fetch the admin kubeconfig using:")
		log.Infof("     " + phase.Colorize.Cyan("k0sctl kubeconfig").String())
	}

	return nil
}
