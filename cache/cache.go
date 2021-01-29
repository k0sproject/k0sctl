package cache

import (
	"fmt"
	"os"
	"path"
)

// EnsureDir makes a directory if it doesn't exist
func EnsureDir(dir string) error {
	err := os.MkdirAll(dir, 0755)

	if err == nil || os.IsExist(err) {
		return nil
	}
	return err
}

func File(parts ...string) (string, error) {
	parts = append([]string{Dir()}, parts...)
	fpath := path.Join(parts...)
	dir := path.Dir(fpath)
	if err := EnsureDir(dir); err != nil {
		return "", err
	}
	return fpath, nil
}

func GetFile(parts ...string) (string, error) {
	fpath, err := File(parts...)
	if err != nil {
		return "", err
	}

	stat, err := os.Stat(fpath)
	if os.IsNotExist(err) {
		return fpath, err
	}

	if stat.Size() == 0 {
		return fpath, fmt.Errorf("cached file size is 0")
	}

	return fpath, nil
}

func GetOrCreate(create func(string) error, parts ...string) (string, error) {
	fpath, err := GetFile(parts...)
	if err != nil {
		err := EnsureDir(path.Dir(fpath))
		if err != nil {
			return "", err
		}
		if err := create(fpath); err != nil {
			return "", err
		}
	}

	return fpath, nil
}
