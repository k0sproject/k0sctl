package phase

import (
	"bytes"
	"context"
	"fmt"
	"path"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/v2/remotefs"
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
	return p.RestoreFrom != "" && p.leader.Metadata.K0sRunningVersion == nil && !p.leader.Reset
}

// Prepare the phase
func (p *Restore) Prepare(config *v1beta1.Cluster) error {
	p.Config = config

	if p.RestoreFrom == "" {
		return nil
	}

	// defined in backup.go
	if p.Config.Spec.K0s.Version.LessThan(backupSinceVersion) {
		return fmt.Errorf("the version of k0s on the host does not support restoring backups")
	}

	p.leader = p.Config.Spec.K0sLeader()

	log.Tracef("restore leader: %s", p.leader)
	log.Tracef("restore leader state: %+v", p.leader.Metadata)
	return nil
}

// Run the phase
func (p *Restore) Run(ctx context.Context) error {
	// Push the backup file to controller
	h := p.leader
	tmpDir, err := h.FS().MkdirTemp("", "")
	if err != nil {
		return err
	}
	dstFile := path.Join(tmpDir, "k0s_backup.tar.gz")
	if err := remotefs.Upload(h.FS(), p.RestoreFrom, dstFile, remotefs.WithPermissions(0o600)); err != nil {
		return err
	}

	defer func() {
		if err := h.Sudo().FS().Remove(dstFile); err != nil {
			log.Warnf("%s: failed to remove backup file %s: %s", h, dstFile, err)
		}

		if err := h.Sudo().FS().Remove(tmpDir); err != nil {
			log.Warnf("%s: failed to remove backup temp dir %s: %s", h, tmpDir, err)
		}
	}()

	// Run restore
	log.Infof("%s: restoring cluster state", h)
	var stdout, stderr bytes.Buffer
	proc := h.Sudo().Proc(h.K0sRestoreCommand(dstFile))
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	waiter, err := proc.Start(ctx)
	if err != nil {
		return fmt.Errorf("run restore: %w", err)
	}

	if err := waiter.Wait(); err != nil {
		log.Debugf("%s: restore stdout: %s", h, stdout.String())
		log.Errorf("%s: restore failed: %s", h, stderr.String())
		return fmt.Errorf("restore failed: %w", err)
	}

	return nil
}
