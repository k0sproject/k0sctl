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

func TestDigMapping(t *testing.T) {
	m := Mapping{
		"foo": Mapping{
			"bar": "foobar",
		},
	}

	assert.Equal(t, "foobar", m.DigMapping("foo")["bar"])

	m.DigMapping("foo", "baz")["dog"] = 1
	assert.Equal(t, 1, m.Dig("foo", "baz", "dog"))
	// Make sure foo.bar was left intact
	assert.Equal(t, "foobar", m.Dig("foo", "bar"))

	// Overwrite foo.bar with a new mapping
	m.DigMapping("foo", "bar")["baz"] = "hello"
	assert.Equal(t, "hello", m.Dig("foo", "bar", "baz"))
}
