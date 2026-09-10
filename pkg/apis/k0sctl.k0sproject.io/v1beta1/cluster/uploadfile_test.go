package cluster

import (
	"os"
	"path/filepath"
	"strings"
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

func TestUploadFileResolveRelativeURLSetsDestination(t *testing.T) {
	u := &UploadFile{Source: "https://example.com/assets/app.tar.gz", DestinationDir: "/opt"}
	require.NoError(t, u.ResolveRelativeTo(""))
	require.Equal(t, "/opt/app.tar.gz", u.DestinationFile)
	require.Empty(t, u.Base)
	require.Empty(t, u.Sources)
}

func TestUploadFileResolveRelativeSingleFile(t *testing.T) {
	tmp := filepath.ToSlash(t.TempDir())
	filePath := filepath.Join(tmp, "a.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("a"), 0o640))

	u := &UploadFile{Source: "a.txt"}
	require.NoError(t, u.ResolveRelativeTo(tmp))
	require.Equal(t, tmp, u.Base)
	require.Len(t, u.Sources, 1)
	require.Equal(t, "a.txt", u.Sources[0].Path)
}

func TestUploadFileValidateSha256(t *testing.T) {
	const sum = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	t.Run("hex sum", func(t *testing.T) {
		u := UploadFile{Source: "https://example.com/a.tar", DestinationFile: "/tmp/a.tar", Sha256: sum}
		require.NoError(t, u.Validate())
	})

	t.Run("uppercase hex sum", func(t *testing.T) {
		u := UploadFile{Source: "https://example.com/a.tar", DestinationFile: "/tmp/a.tar", Sha256: strings.ToUpper(sum)}
		require.NoError(t, u.Validate())
	})

	t.Run("too short", func(t *testing.T) {
		u := UploadFile{Source: "https://example.com/a.tar", DestinationFile: "/tmp/a.tar", Sha256: sum[:63]}
		require.ErrorContains(t, u.Validate(), "must be a hex encoded sha256 sum")
	})

	t.Run("not hex", func(t *testing.T) {
		u := UploadFile{Source: "https://example.com/a.tar", DestinationFile: "/tmp/a.tar", Sha256: strings.Repeat("z", 64)}
		require.ErrorContains(t, u.Validate(), "must be a hex encoded sha256 sum")
	})

	t.Run("with inline data", func(t *testing.T) {
		u := UploadFile{Data: "hello", DestinationFile: "/tmp/a.txt", Sha256: sum}
		require.ErrorContains(t, u.Validate(), "sha256 can not be used with inline data")
	})
}

func TestUploadFileResolveGlobRejectsSha256(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("b"), 0o644))

	u := &UploadFile{
		Source:         "*.yaml",
		DestinationDir: "/tmp",
		Sha256:         "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
	require.ErrorContains(t, u.ResolveRelativeTo(filepath.ToSlash(dir)), "can only describe a single file")
}
