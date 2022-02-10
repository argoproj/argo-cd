package askpass

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	s := NewServer()
	id := s.Add("foo", "bar")

	assert.Equal(t, "foo", s.creds[id].Username)
	assert.Equal(t, "bar", s.creds[id].Password)
}

func TestRemove(t *testing.T) {
	s := NewServer()
	s.creds["some-id"] = Creds{Username: "foo"}

	s.Remove("some-id")

	_, ok := s.creds["some-id"]
	assert.False(t, ok)
}
