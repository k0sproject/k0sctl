// +build windows
package cache

import (
	"path"

	"golang.org/x/sys/windows"
)

// Directory where k0sctl temporary files should be stored
func Dir() string {
	return path.Join(windows.KnownFolderPath(windows.FOLDERID_CSIDL_LOCAL_APPDATA, 0), "k0sctl")
}
