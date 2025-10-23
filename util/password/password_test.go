package password

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPasswordHasher(t *testing.T, h PasswordHasher) {
	t.Helper()
	// Use the default work factor
	const (
		defaultPassword = "Hello, world!"
		pollution       = "extradata12345"
	)
	hashedPassword, _ := h.HashPassword(defaultPassword)
	assert.True(t, h.VerifyPassword(defaultPassword, hashedPassword), "Password %q should have validated against hash %q", defaultPassword, hashedPassword)
	assert.False(t, h.VerifyPassword(defaultPassword, pollution+hashedPassword), "Password %q should NOT have validated against hash %q", defaultPassword, pollution+hashedPassword)
}

func TestBcryptPasswordHasher(t *testing.T) {
	// Use the default work factor
	h := BcryptPasswordHasher{0}
	testPasswordHasher(t, h)
}

func TestDummyPasswordHasher(t *testing.T) {
	h := DummyPasswordHasher{}
	testPasswordHasher(t, h)
}

func TestPasswordHashing(t *testing.T) {
	const (
		defaultPassword = "Hello, world!"
		blankPassword   = ""
	)
	hashers := []PasswordHasher{
		BcryptPasswordHasher{0},
		DummyPasswordHasher{},
	}

	hashedPassword, _ := hashPasswordWithHashers(defaultPassword, hashers)
	valid, stale := verifyPasswordWithHashers(defaultPassword, hashedPassword, hashers)
	assert.True(t, valid, "Password %q should have validated against hash %q", defaultPassword, hashedPassword)
	assert.False(t, stale, "Password %q should not have been marked stale against hash %q", defaultPassword, hashedPassword)
	valid, stale = verifyPasswordWithHashers(defaultPassword, defaultPassword, hashers)
	assert.True(t, valid, "Password %q should have validated against itself with dummy hasher", defaultPassword)
	assert.True(t, stale, "Password %q should have been acknowledged stale against itself with dummy hasher", defaultPassword)

	hashedPassword, err := hashPasswordWithHashers(blankPassword, hashers)
	require.Error(t, err, "Blank password should have produced error, rather than hash %q", hashedPassword)

	valid, _ = verifyPasswordWithHashers(blankPassword, "", hashers)
	assert.False(t, valid, "Blank password should have failed verification")
}
