package phase

import (
	"fmt"
	"path"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

type Restore struct {
	GenericPhase

	RestoreFrom string
	leader      *cluster.Host
}

// Title for the phase
func (p *Restore) Title() string {
	return "Restore cluster state"
}

// ShouldRun is true when there path to backup file
func (p *Restore) ShouldRun() bool {
	return p.RestoreFrom != "" && p.leader.Metadata.K0sRunningVersion == ""
}

// Prepare the phase
func (p *Restore) Prepare(config *v1beta1.Cluster) error {
	log.Tracef("restore from: %s", p.RestoreFrom)
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()

	if p.RestoreFrom != "" && p.leader.Exec(p.leader.Configurer.K0sCmdf("restore --help"), exec.Sudo(p.leader)) != nil {
		return fmt.Errorf("the version of k0s on the host does not support restoring backups")
	}

	log.Tracef("restore leader: %s", p.leader)
	log.Tracef("restore leader state: %+v", p.leader.Metadata)
	return nil
}

// Run the phase
func (p *Restore) Run() error {
	// Push the backup file to controller
	h := p.leader
	tmpDir, err := h.Configurer.TempDir(h)
	if err != nil {
		return err
	}
	dstFile := path.Join(tmpDir, "k0s_backup.tar.gz")
	if err := h.Upload(p.RestoreFrom, dstFile); err != nil {
		return err
	}

	defer func() {
		if err := h.Configurer.DeleteFile(h, dstFile); err != nil {
			log.Warnf("%s: failed to remove backup file %s: %s", h, dstFile, err)
		}

		if err := h.Configurer.DeleteDir(h, tmpDir, exec.Sudo(h)); err != nil {
			log.Warnf("%s: failed to remove backup temp dir %s: %s", h, tmpDir, err)
		}
	}()

	// Run restore
	log.Infof("%s: restoring cluster state", h)
	if err := h.Exec(h.K0sRestoreCommand(dstFile), exec.Sudo(h)); err != nil {
		return err
	}

	return nil
}
