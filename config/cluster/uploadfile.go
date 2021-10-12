package cluster

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// UploadFile describes a file to be uploaded for the host
type UploadFile struct {
	Name           string      `yaml:"name,omitempty"`
	Source         string      `yaml:"src" validate:"required"`
	DestinationDir string      `yaml:"dstDir" validate:"required"`
	PermMode       interface{} `yaml:"perm" default:"0755"`
	PermString     string      `yaml:"-"`
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (u *UploadFile) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type uploadFile UploadFile
	yu := (*uploadFile)(u)

	if err := unmarshal(yu); err != nil {
		return err
	}

	switch t := u.PermMode.(type) {
	case int:
		if t < 0 {
			return fmt.Errorf("invalid uploadFile permission: %d: must be a positive value", t)
		}
		if t == 0 {
			return fmt.Errorf("invalid nil uploadFile permission")
		}
		u.PermString = fmt.Sprintf("%#o", t)
	case string:
		u.PermString = t
	default:
		return fmt.Errorf("invalid value for uploadFile perm, must be a string or a number")
	}

	for i, c := range u.PermString {
		n, err := strconv.Atoi(string(c))
		if err != nil {
			return fmt.Errorf("failed to parse uploadFile permission %s: %w", u.PermString, err)
		}

		// These could catch some weird octal conversion mistakes
		if i == 1 && n < 4 {
			return fmt.Errorf("invalid uploadFile permission %s: owner would have unconventional access", u.PermString)
		}
		if n > 7 {
			return fmt.Errorf("invalid uploadFile permission %s: octal value can't have numbers over 7", u.PermString)
		}
	}

	return nil
}

func (u *UploadFile) Resolve() ([]string, error) {
	sources, err := filepath.Glob(u.Source)
	if err != nil {
		return nil, err
	}
	return sources, nil
}
