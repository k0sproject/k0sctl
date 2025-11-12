package cluster

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestPermStringUnmarshalWithOctal(t *testing.T) {
	u := UploadFile{}
	yml := []byte(`
src: .
dstDir: .
perm: 0755
`)

	require.NoError(t, yaml.Unmarshal(yml, &u))
	require.Equal(t, "0755", u.PermString)
}

func TestPermStringUnmarshalWithString(t *testing.T) {
	u := UploadFile{}
	yml := []byte(`
src: .
dstDir: .
perm: "0755"
`)

	require.NoError(t, yaml.Unmarshal(yml, &u))
	require.Equal(t, "0755", u.PermString)
}

func TestPermStringUnmarshalWithInvalidString(t *testing.T) {
	u := UploadFile{}
	yml := []byte(`
src: .
dstDir: .
perm: u+rwx
`)

	require.Error(t, yaml.Unmarshal(yml, &u))
}

func TestPermStringUnmarshalWithInvalidNumber(t *testing.T) {
	u := UploadFile{}
	yml := []byte(`
src: .
dstDir: .
perm: 0800
`)

	require.Error(t, yaml.Unmarshal(yml, &u))
}

func TestPermStringUnmarshalWithZero(t *testing.T) {
	u := UploadFile{}
	yml := []byte(`
src: .
dstDir: .
perm: 0
`)

	require.Error(t, yaml.Unmarshal(yml, &u))
}

func TestUploadFileValidateRequiresDestinationFileForData(t *testing.T) {
	u := UploadFile{Data: "hello", DestinationDir: "/tmp"}

	err := u.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "name or dst required for data")
}

func TestUploadFileValidateDataWithDestinationFile(t *testing.T) {
	u := UploadFile{Data: "hello", DestinationFile: "/tmp/inline.txt"}

	require.NoError(t, u.Validate())
}

func TestUploadFileValidateRequiresSourceOrData(t *testing.T) {
	u := UploadFile{Data: "   "}

	err := u.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "src or data required")
}

func TestUploadFileValidateRequiresDestinationFileOrName(t *testing.T) {
	u := UploadFile{Data: "hello", DestinationDir: "/tmp/"}

	err := u.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "name or dst required for data")
}

func TestUploadFileResolveURLSetsDst(t *testing.T) {
	u := &UploadFile{Source: "https://example.com/assets/app.tar.gz", DestinationDir: "/opt"}
	require.NoError(t, u.Resolve("/tmp/config.yaml"))
	require.Equal(t, "/opt/app.tar.gz", u.DestinationFile)
	require.Equal(t, "", u.Base)
	require.Len(t, u.Sources, 0)
}

func TestUploadFileResolveLocalSingleFile(t *testing.T) {
	tmp := t.TempDir()
	fp := path.Join(tmp, "a.txt")
	require.NoError(t, os.WriteFile(fp, []byte("a"), 0o640))

	u := &UploadFile{Source: "a.txt"}
	require.NoError(t, u.Resolve(path.Join(tmp, "cfg.yaml")))
	require.Equal(t, tmp, u.Base)
	require.Len(t, u.Sources, 1)
	require.Equal(t, "a.txt", u.Sources[0].Path)
}
