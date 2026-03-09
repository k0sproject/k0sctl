package binprovider

import (
	"context"

	"github.com/k0sproject/k0sctl/pkg/k0s"
	log "github.com/sirupsen/logrus"
)

// localFile uploads a developer-supplied k0s binary from the local machine to the host.
type localFile struct {
	stagedFile
	localPath   string
	installPath string
	changed     func() bool
}

func (p *localFile) IsUpload() bool { return true }

func (p *localFile) NeedsUpgrade() bool {
	result := p.changed()
	log.Debugf("%s: localFile provider: file changed=%v → needsUpgrade=%v", p.host, result, result)
	return result
}

func (p *localFile) Stage(_ context.Context) (string, error) {
	tmp, err := stageUpload(p.host, p.localPath, p.installPath)
	if err != nil {
		return "", err
	}
	p.tmpPath = tmp
	return tmp, nil
}

// NewLocalFile returns a BinaryProvider that uploads the binary at localPath to the host.
// changed is called by NeedsUpgrade to determine whether the local file differs from the
// remote install location (e.g. by comparing size and mtime).
func NewLocalFile(h Host, localPath, installPath string, changed func() bool) k0s.BinaryProvider {
	return &localFile{stagedFile: stagedFile{host: h}, localPath: localPath, installPath: installPath, changed: changed}
}
