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
