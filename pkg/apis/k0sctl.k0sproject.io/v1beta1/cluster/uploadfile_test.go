package cluster

import (
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

func TestUploadFileValidateRequiresDestinationFileForContent(t *testing.T) {
	u := UploadFile{Content: "hello", DestinationDir: "/tmp"}

	err := u.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "name or dst required for content")
}

func TestUploadFileValidateContentWithDestinationFile(t *testing.T) {
	u := UploadFile{Content: "hello", DestinationFile: "/tmp/inline.txt"}

	require.NoError(t, u.Validate())
}

func TestUploadFileValidateRequiresSourceOrContent(t *testing.T) {
	u := UploadFile{Content: "   "}

	err := u.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "src or content required")
}

func TestUploadFileValidateRequiresDestinationFileOrName(t *testing.T) {
	u := UploadFile{Content: "hello", DestinationDir: "/tmp/"}

	err := u.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "name or dst required for content")
}
