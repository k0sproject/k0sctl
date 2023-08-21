package phase

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
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
func (p *Backup) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	leader := p.Config.Spec.K0sLeader()
	if leader.Metadata.K0sRunningVersion == "" {
		return fmt.Errorf("failed to find a running controller")
	}

	if leader.Exec(leader.K0sCmdf("backup --help"), exec.Sudo(leader)) != nil {
		return fmt.Errorf("the version of k0s on the host does not support taking backups")
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

	if err := h.Exec(h.K0sBackupCommand(backupDir), exec.Sudo(h)); err != nil {
		return err
	}

	// get the name of the backup file
	remoteFile, err := h.ExecOutputf(`ls "%s"`, backupDir)
	if err != nil {
		return err
	}
	remotePath := path.Join(backupDir, remoteFile)

	defer func() {
		log.Debugf("%s: cleaning up %s", h, remotePath)
		if err := h.Configurer.DeleteFile(h, remotePath); err != nil {
			log.Warnf("%s: failed to clean up backup temp file %s: %s", h, remotePath, err)
		}
		if err := h.Configurer.DeleteDir(h, backupDir, exec.Sudo(h)); err != nil {
			log.Warnf("%s: failed to clean up backup temp directory %s: %s", h, backupDir, err)
		}
	}()

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

	if err := h.Execf(`cat "%s"`, remotePath, exec.Writer(f)); err != nil {
		return err
	}

	log.Infof("backup file written to %s", localFile)
	return nil
}
