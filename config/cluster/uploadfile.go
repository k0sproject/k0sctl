package cluster

import (
	"fmt"
	"path/filepath"
)

// UploadFile describes a file to be uploaded for the host
type UploadFile struct {
	Name        string `yaml:"name,omitempty"`
	Source      string `yaml:"src" validate:"required"`
	Destination string `yaml:"dst" validate:"required"`
	PermMode    string `yaml:"perm" default:"0755"`
}

type FileUploadInfo struct {
	Source      string
	Destination string
}

func (u *UploadFile) Resolve() ([]FileUploadInfo, error) {
	sources, err := filepath.Glob(u.Source)
	if err != nil {
		return nil, err
	}
	return u.resolveFileUploadInfos(sources)
}

func (u *UploadFile) resolveFileUploadInfos(sources []string) ([]FileUploadInfo, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("failed to resolve upload source to any files")
	}

	// For single files we expect the dest to have full path for the upload target
	if len(sources) == 1 {
		return []FileUploadInfo{{
			Source:      sources[0],
			Destination: u.Destination,
		}}, nil
	}

	infos := make([]FileUploadInfo, len(sources))
	for idx, s := range sources {
		target := filepath.Join(u.Destination, filepath.Base(s))
		infos[idx] = FileUploadInfo{
			Source:      s,
			Destination: target,
		}
	}

	return infos, nil
}
