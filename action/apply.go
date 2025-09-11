package action

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0sproject/k0sctl/phase"

	log "github.com/sirupsen/logrus"
)

type ApplyOptions struct {
	// Manager is the phase manager
	Manager *phase.Manager
	// DisableDowngradeCheck skips the downgrade check
	DisableDowngradeCheck bool
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
	// KubeconfigUser is the username to use in the kubeconfig
	KubeconfigUser string
	// KubeconfigCluster is the cluster name to use in the kubeconfig
	KubeconfigCluster string
	// ConfigPaths is the list of paths to the configuration files (used for kubeconfig command tip on success)
	ConfigPaths []string
}

type Apply struct {
	ApplyOptions
	Phases phase.Phases
}

// NewApply creates a new Apply action. The list of phases can be modified via the Phases field, for example:
//
//	apply := NewApply(opts)
//	gatherK0sFacts := &phase.GatherK0sFacts{} // advisable to get the title from the phase itself instead of hardcoding the title.
//	apply.Phases.InsertBefore(gatherK0sFacts.Title(), &myCustomPhase{}) // insert a custom phase before the GatherK0sFacts phase
func NewApply(opts ApplyOptions) *Apply {
	// lockPhase := &phase.Lock{}
	// unlockPhase := lockPhase.UnlockPhase()
	apply := &Apply{
		ApplyOptions: opts,
		Phases: phase.Phases{
			&phase.DefaultK0sVersion{},
			&phase.Connect{},
			&phase.DetectOS{},
			// lockPhase,
			&phase.PrepareHosts{},
			&phase.GatherFacts{},
			&phase.ValidateHosts{},
			&phase.GatherK0sFacts{},
			&phase.ValidateFacts{SkipDowngradeCheck: opts.DisableDowngradeCheck},
			&phase.ValidateEtcdMembers{},

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
				RestoreFrom: opts.RestoreFrom,
			},
			&phase.RunHooks{Stage: "before", Action: "apply"},
			&phase.InitializeK0s{},
			&phase.InstallControllers{},
			&phase.InstallWorkers{},
			&phase.UpgradeControllers{},
			&phase.UpgradeWorkers{NoDrain: opts.NoDrain},
			&phase.Reinstall{},
			&phase.ResetWorkers{NoDrain: opts.NoDrain},
			&phase.ResetControllers{NoDrain: opts.NoDrain},
			&phase.RunHooks{Stage: "after", Action: "apply"},
			&phase.ApplyManifests{},
			// unlockPhase,
		},
	}
	if opts.KubeconfigOut != nil {
		apply.Phases = append(apply.Phases, &phase.GetKubeconfig{APIAddress: opts.KubeconfigAPIAddress, User: opts.KubeconfigUser, Cluster: opts.KubeconfigCluster})
	}
	apply.Phases = append(apply.Phases, &phase.Disconnect{})

	return apply
}

// Run the Apply action
func (a Apply) Run(ctx context.Context) error {
	if len(a.Phases) == 0 {
		// for backwards compatibility with the old Apply struct without NewApply(..)
		tmpApply := NewApply(a.ApplyOptions)
		a.Phases = tmpApply.Phases
	}
	start := time.Now()

	phase.NoWait = a.NoWait

	a.Manager.SetPhases(a.Phases)

	var result error

	if result = a.Manager.Run(ctx); result != nil {
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
	log.Info(phase.Colorize.Green(text).String())

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
		cmd := &strings.Builder{}
		executable, err := os.Executable()
		if err != nil {
			executable = "k0sctl"
		} else {
			// check if the basename of executable is in the PATH, if so, just use the basename
			if _, err := exec.LookPath(filepath.Base(executable)); err == nil {
				executable = filepath.Base(executable)
			}
		}

		cmd.WriteString(executable)
		cmd.WriteString(" kubeconfig")

		if len(a.ConfigPaths) > 0 && (len(a.ConfigPaths) != 1 && a.ConfigPaths[0] != "-" && a.ConfigPaths[0] != "k0sctl.yaml") {
			for _, path := range a.ConfigPaths {
				cmd.WriteString(" --config ")
				cmd.WriteString(path)
			}
		}

		log.Info("Tip: To access the cluster you can now fetch the admin kubeconfig using:")
		log.Info("     " + phase.Colorize.Cyan(cmd.String()).String())
	}

	return nil
}
