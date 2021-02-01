package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDig(t *testing.T) {
	m := Mapping{
		"foo": Mapping{
			"bar": "foobar",
		},
	}

	assert.Equal(t, "foobar", m.Dig("foo", "bar"))
	assert.Equal(t, "foobar", m.DigString("foo", "bar"))

	assert.Nil(t, m.Dig("foo", "non-existing", "key"))
}

func TestDeepSet(t *testing.T) {
	m := Mapping{
		"foo": Mapping{
			"bar": "foobar",
		},
	}

	m.DeepSet([]string{"bar", "baz", "dog"}, 1)
	assert.Equal(t, 1, m.Dig("bar", "baz", "dog"))

	m.DeepSet([]string{"foo", "baz", "dog"}, "hello")
	assert.Equal(t, "hello", m.Dig("foo", "baz", "dog"))
	assert.Equal(t, "foobar", m.Dig("foo", "bar"))

	m.DeepSet([]string{"foo", "bar", "baz"}, "hello")
	assert.Nil(t, m.Dig("foo", "bar", "baz"))
	assert.Equal(t, "foobar", m.Dig("foo", "bar"))
}
