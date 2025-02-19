package phase

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

var _ Phase = &Backup{}

var backupSinceVersion = version.MustParse("v1.21.0-rc.1+k0s.0")

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

	if !p.Config.Spec.K0s.Version.GreaterThanOrEqual(backupSinceVersion) {
		return fmt.Errorf("the version of k0s on the host does not support taking backups")
	}

	leader := p.Config.Spec.K0sLeader()
	if leader.Metadata.K0sRunningVersion == nil {
		return fmt.Errorf("failed to find a running controller")
	}

	p.leader = leader
	p.leader.Metadata.IsK0sLeader = true
	return nil
}

// ShouldRun is true when there is a leader host
func (p *Backup) ShouldRun() bool {
	return p.leader != nil
}

// Run the phase
func (p *Backup) Run(_ context.Context) error {
	h := p.leader

	log.Infof("%s: backing up", h)
	var backupDir string
	err := p.Wet(h, "create a tempdir using `mktemp -d`", func() error {
		b, err := h.Configurer.TempDir(h)
		if err != nil {
			return err
		}
		backupDir = b
		return nil
	}, func() error {
		backupDir = "/tmp/k0s_backup.dryrun"
		return nil
	})
	if err != nil {
		return err
	}

	cmd := h.K0sBackupCommand(backupDir)
	err = p.Wet(h, fmt.Sprintf("create backup using `%s`", cmd), func() error {
		return h.Exec(h.K0sBackupCommand(backupDir), exec.Sudo(h))
	})
	if err != nil {
		return err
	}

	// get the name of the backup file
	var remoteFile string
	if p.IsWet() {
		r, err := h.ExecOutputf(`ls "%s"`, backupDir)
		if err != nil {
			return err
		}
		remoteFile = r
	} else {
		remoteFile = "k0s_backup.dryrun.tar.gz"
	}
	remotePath := path.Join(backupDir, remoteFile)

	defer func() {
		if p.IsWet() {
			log.Debugf("%s: cleaning up %s", h, remotePath)
			if err := h.Configurer.DeleteFile(h, remotePath); err != nil {
				log.Warnf("%s: failed to clean up backup temp file %s: %s", h, remotePath, err)
			}
			if err := h.Configurer.DeleteDir(h, backupDir, exec.Sudo(h)); err != nil {
				log.Warnf("%s: failed to clean up backup temp directory %s: %s", h, backupDir, err)
			}
		} else {
			p.DryMsg(h, "delete the tempdir")
		}
	}()

	localFile, err := filepath.Abs(fmt.Sprintf("k0s_backup_%d.tar.gz", time.Now().Unix()))
	if err != nil {
		return err
	}

	if p.IsWet() {
		// Download the file
		f, err := os.OpenFile(localFile, os.O_RDWR|os.O_CREATE|os.O_SYNC, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := h.Execf(`cat "%s"`, remotePath, exec.Writer(f)); err != nil {
			return err
		}

		log.Infof("backup file written to %s", localFile)
	} else {
		p.DryMsgf(nil, "download the backup file to local host as %s", localFile)
	}
	return nil
}
