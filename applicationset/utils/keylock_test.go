package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyLock_Delete(t *testing.T) {
	k := NewKeyLock()
	k.Lock("foo")
	k.Delete("foo")
	// non-blocking because the key is deleted
	k.Lock("foo")()
}

func TestKeyLock_Lock(t *testing.T) {
	k := NewKeyLock()
	count := 0
	k.Lock("foo")
	go func() {
		// blocking
		k.Lock("foo")()
		count++
	}()

	k.Lock("bar")
	assert.Equal(t, count, 0)
}
