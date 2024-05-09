package action

import (
	"fmt"
	"io"
	"time"

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

		// if UploadBinaries: true
		&phase.DownloadBinaries{}, // downloads k0s binaries to local cache
		&phase.UploadK0s{},        // uploads k0s binaries to hosts from cache

		// if UploadBinaries: false
		&phase.DownloadK0s{}, // downloads k0s binaries directly from hosts

		&phase.UploadFiles{},
		&phase.InstallBinaries{},
		&phase.PrepareArm{},
		&phase.ConfigureK0s{},
		&phase.Restore{
			RestoreFrom: a.RestoreFrom,
		},
		&phase.RunHooks{Stage: "before", Action: "apply"},
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

	var result error

	if result = a.Manager.Run(); result != nil {
		log.Info(phase.Colorize.Red("==> Apply failed").String())
		return result
	}

	if a.KubeconfigOut != nil {
		if _, err := a.KubeconfigOut.Write([]byte(a.Manager.Config.Metadata.Kubeconfig)); err != nil {
			log.Warnf("failed to write kubeconfig to %s: %v", a.KubeconfigOut, err)
		}
	}

	duration := time.Since(start).Truncate(time.Second)
	text := fmt.Sprintf("==> Finished in %s", duration)
	log.Infof(phase.Colorize.Green(text).String())

	for _, host := range a.Manager.Config.Spec.Hosts {
		if host.Reset {
			log.Info("There were nodes that got uninstalled during the apply phase. Please remove them from your k0sctl config file")
			break
		}
	}

	if !a.Manager.DryRun {
		log.Infof("k0s cluster version %s is now installed", a.Manager.Config.Spec.K0s.Version)
	}

	if a.KubeconfigOut == nil {
		log.Infof("Tip: To access the cluster you can now fetch the admin kubeconfig using:")
		log.Infof("     " + phase.Colorize.Cyan("k0sctl kubeconfig").String())
	}

	return nil
}
