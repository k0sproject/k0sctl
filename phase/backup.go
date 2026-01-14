package phase

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"

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

	Out io.Writer

	leader *cluster.Host
}

// Title returns the title for the phase
func (p *Backup) Title() string {
	return "Take backup"
}

// Before runs "before backup" hooks
func (p *Backup) Before() error {
	if err := p.runHooks(context.Background(), "backup", "before", p.leader); err != nil {
		return fmt.Errorf("running hooks failed: %w", err)
	}
	return nil
}

// After runs "after backup" hooks
func (p *Backup) After() error {
	if err := p.runHooks(context.Background(), "backup", "after", p.leader); err != nil {
		return fmt.Errorf("running hooks failed: %w", err)
	}
	return nil
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
		entries, err := fs.ReadDir(h.SudoFsys(), backupDir)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return fmt.Errorf("no backup file found in %s", backupDir)
		}
		remoteFile = entries[0].Name()
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

	if p.IsWet() {
		f, err := h.SudoFsys().Open(remotePath)
		if err != nil {
			return fmt.Errorf("open backup for streaming: %w", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				log.Warnf("%s: failed to close backup file %s: %v", h, remotePath, err)
			}
		}()
		if _, err := io.Copy(p.Out, f); err != nil {
			return fmt.Errorf("download backup: %w", err)
		}
	} else {
		p.DryMsgf(nil, "download the backup file to local host")
	}
	return nil
}
