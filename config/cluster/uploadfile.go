package cluster

import (
	"path/filepath"
)

// UploadFile describes a file to be uploaded for the host
type UploadFile struct {
	Name           string `yaml:"name,omitempty"`
	Source         string `yaml:"src" validate:"required"`
	DestinationDir string `yaml:"dstDir" validate:"required"`
	PermMode       string `yaml:"perm" default:"0755"`
}

func (u *UploadFile) Resolve() ([]string, error) {
	sources, err := filepath.Glob(u.Source)
	if err != nil {
		return nil, err
	}
	return sources, nil
}
