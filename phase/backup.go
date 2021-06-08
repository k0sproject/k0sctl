package phase

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

var _ phase = &Backup{}

// Backup connect to one of the controllers and takes a backup
type Backup struct {
	GenericPhase

	leader *cluster.Host
}

// Title returns the title for the phase
func (p *Backup) Title() string {
	return "Take backup"
}

// Prepare the phase
func (p *Backup) Prepare(config *config.Cluster) error {
	p.Config = config
	leader := p.Config.Spec.K0sLeader()
	if leader.Metadata.K0sRunningVersion == "" {
		return fmt.Errorf("failed to find a running controller")
	}
	p.leader = leader
	return nil
}

// ShouldRun is true when there is a leader host
func (p *Backup) ShouldRun() bool {
	return p.leader != nil
}

// Run the phase
func (p *Backup) Run() error {
	h := p.leader
	h.Metadata.IsK0sLeader = true

	log.Infof("%s: backing up", h)
	backupDir, err := h.Configurer.TempDir(h)
	if err != nil {
		return err
	}

	if err := h.Exec(h.K0sBackupCommand(backupDir)); err != nil {
		return err
	}

	// get the name of the backup file
	remoteFile, err := h.ExecOutput(fmt.Sprintf("ls %s", backupDir))
	if err != nil {
		return err
	}

	localFile, err := filepath.Abs(fmt.Sprintf("k0s_backup_%d.tar.gz", time.Now().Unix()))
	if err != nil {
		return err
	}

	// Download the file
	f, err := os.OpenFile(localFile, os.O_RDWR|os.O_CREATE|os.O_SYNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := fmt.Sprintf("cat %s/%s", backupDir, remoteFile)
	if err := h.Exec(cmd, exec.Writer(f)); err != nil {
		return err
	}

	log.Infof("backup file written to %s", localFile)
	return nil
}
