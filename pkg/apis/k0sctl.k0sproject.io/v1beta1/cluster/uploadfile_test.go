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
