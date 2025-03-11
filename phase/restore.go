package phase

import (
	"bytes"
	"context"
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
func (p *Restore) Run(_ context.Context) error {
	// Push the backup file to controller
	h := p.leader
	tmpDir, err := h.Configurer.TempDir(h)
	if err != nil {
		return err
	}
	dstFile := path.Join(tmpDir, "k0s_backup.tar.gz")
	if err := h.Upload(p.RestoreFrom, dstFile, 0o600, exec.LogError(true)); err != nil {
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
	var stdout, stderr bytes.Buffer
	cmd, err := h.ExecStreams(h.K0sRestoreCommand(dstFile), nil, &stdout, &stderr, exec.Sudo(h))
	if err != nil {
		return fmt.Errorf("run restore: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		log.Debugf("%s: restore stdout: %s", h, stdout.String())
		log.Errorf("%s: restore failed: %s", h, stderr.String())
		return fmt.Errorf("restore failed: %w", err)
	}

	return nil
}
