package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ResolveFileUploads(t *testing.T) {
	t.Run("single source assumes target is a file", func(t *testing.T) {
		uploads := UploadFile{
			Source:      "/path/to/single/file",
			Destination: "/foo/bar",
		}
		infos, err := uploads.resolveFileUploadInfos([]string{"/x/y/z"})
		require.NoError(t, err)
		require.Equal(t, FileUploadInfo{
			Source:      "/x/y/z",
			Destination: "/foo/bar",
		}, infos[0])
	})

	t.Run("many sources assumes target is a dir", func(t *testing.T) {
		uploads := UploadFile{
			Source:      "/glob/resolving/multiple/files",
			Destination: "/foo/bar",
		}
		infos, err := uploads.resolveFileUploadInfos([]string{"/x/y/z", "/z/y/x"})
		require.NoError(t, err)
		require.Len(t, infos, 2)
		require.Contains(t, infos, FileUploadInfo{
			Source:      "/x/y/z",
			Destination: "/foo/bar/z",
		})
		require.Contains(t, infos, FileUploadInfo{
			Source:      "/z/y/x",
			Destination: "/foo/bar/x",
		})

	})

	t.Run("no sources is error", func(t *testing.T) {
		uploads := UploadFile{}

		_, err := uploads.resolveFileUploadInfos([]string{})
		require.Error(t, err)

		_, err = uploads.resolveFileUploadInfos(nil)
		require.Error(t, err)
	})
}
