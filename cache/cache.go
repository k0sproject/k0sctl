package cache

import (
	"fmt"
	"os"
	"path"

	"golang.org/x/sys/unix"
)

// EnsureDir makes a directory if it doesn't exist
func EnsureDir(dir string) error {
	err := os.MkdirAll(dir, 0755)

	if err == nil || os.IsExist(err) {
		if unix.Access(dir, unix.W_OK) != nil {
			return fmt.Errorf("not writable: %s", dir)
		}
	}

	return err
}

// File returns a path to a file in the cache dir
func File(parts ...string) string {
	parts = append([]string{Dir()}, parts...)
	return path.Join(parts...)
}

// GetFile returns a file from the cache directory if it exists
func GetFile(parts ...string) (string, error) {
	fpath := File(parts...)

	stat, err := os.Stat(fpath)
	if os.IsNotExist(err) {
		return fpath, err
	}

	if stat.Size() == 0 {
		return fpath, fmt.Errorf("cached file size is 0")
	}

	return fpath, nil
}

// GetOrCreate generates a path and runs the provided function if the file does not exist
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
