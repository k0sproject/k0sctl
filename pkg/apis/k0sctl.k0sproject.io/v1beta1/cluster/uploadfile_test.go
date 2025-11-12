package cluster

import (
	"os"
	"path/filepath"
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

func TestUploadFileResolveRelativeToBaseDir(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "files")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	filePath := filepath.Join(srcDir, "example.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0o644))

	var u UploadFile
	yml := []byte(`
src: files/example.txt
dstDir: /tmp
`)
	require.NoError(t, yaml.Unmarshal(yml, &u))
	require.NoError(t, u.ResolveRelativeTo(filepath.ToSlash(dir)))

	require.Equal(t, filepath.ToSlash(srcDir), u.Base)
	require.Len(t, u.Sources, 1)
	require.Equal(t, "example.txt", u.Sources[0].Path)
}

func TestUploadFileResolveGlobRelativeToBaseDir(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "files", "manifests")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.yaml"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "b.yaml"), []byte("b"), 0o644))

	var u UploadFile
	yml := []byte(`
src: files/**/*.yaml
dstDir: /tmp
`)
	require.NoError(t, yaml.Unmarshal(yml, &u))
	require.NoError(t, u.ResolveRelativeTo(filepath.ToSlash(dir)))

	require.Equal(t, filepath.ToSlash(filepath.Join(dir, "files")), u.Base)
	require.Len(t, u.Sources, 2)
	require.ElementsMatch(t, []string{"manifests/a.yaml", "manifests/b.yaml"}, []string{u.Sources[0].Path, u.Sources[1].Path})
}
