package askpass

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	s := NewServer(SocketPath)
	nonce := s.Add("foo", "bar")

	assert.Equal(t, "foo", s.creds[nonce].Username)
	assert.Equal(t, "bar", s.creds[nonce].Password)
}

func TestRemove(t *testing.T) {
	s := NewServer(SocketPath)
	s.creds["some-id"] = Creds{Username: "foo"}

	s.Remove("some-id")

	_, ok := s.creds["some-id"]
	assert.False(t, ok)
}
