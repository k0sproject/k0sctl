package phase

import (
	"fmt"

	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
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
func (p *Restore) Prepare(config *config.Cluster) error {
	log.Debugf("restore from: %s", p.RestoreFrom)
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()

	if p.RestoreFrom != "" && p.leader.Exec(p.leader.Configurer.K0sCmdf("restore --help")) != nil {
		return fmt.Errorf("the version of k0s on the host does not support restoring backups")
	}

	log.Debugf("restore leader: %s", p.leader)
	log.Debugf("restore leader state: %+v", p.leader.Metadata)
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
	dstFile := fmt.Sprintf("%s/k0s_backup.tar.gz", tmpDir)
	if err := h.Upload(p.RestoreFrom, dstFile); err != nil {
		return err
	}

	// Run restore
	log.Infof("%s: restoring cluster state", h)
	if err := h.Exec(h.K0sRestoreCommand(dstFile)); err != nil {
		return err
	}

	return nil
}
