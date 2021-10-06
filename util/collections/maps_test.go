package collections

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCopyStringMap(t *testing.T) {
	out := CopyStringMap(map[string]string{"foo": "bar"})
	assert.Equal(t, map[string]string{"foo": "bar"}, out)
}

func TestStringMapsEqual(t *testing.T) {
	assert.True(t, StringMapsEqual(nil, nil))
	assert.True(t, StringMapsEqual(nil, map[string]string{}))
	assert.True(t, StringMapsEqual(map[string]string{}, nil))
	assert.True(t, StringMapsEqual(map[string]string{"foo": "bar"}, map[string]string{"foo": "bar"}))
	assert.False(t, StringMapsEqual(map[string]string{"foo": "bar"}, nil))
	assert.False(t, StringMapsEqual(map[string]string{"foo": "bar"}, map[string]string{"foo": "bar1"}))
}
