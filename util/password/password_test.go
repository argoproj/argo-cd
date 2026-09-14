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
	t.Parallel()
	// Use the default work factor
	h := BcryptPasswordHasher{0}
	testPasswordHasher(t, h)
}

func TestPBKDF2PasswordHasher(t *testing.T) {
	t.Parallel()

	h := PBKDF2PasswordHasher{}

	testPasswordHasher(t, h)

	hashedPassword1, err := h.HashPassword("Hello, world!")
	require.NoError(t, err)

	hashedPassword2, err := h.HashPassword("Hello, world!")
	require.NoError(t, err)

	assert.NotEqual(t, hashedPassword1, hashedPassword2, "Password hashes should use different salts")
	assert.True(t, h.VerifyPassword("Hello, world!", hashedPassword1))
	assert.False(t, h.VerifyPassword("wrong password", hashedPassword1))

	assert.False(t, h.VerifyPassword("Hello, world!", "invalid"))
	assert.False(t, h.VerifyPassword("Hello, world!", "pbkdf2-sha256$v1$1$invalid$invalid"))
	assert.False(t, h.VerifyPassword("Hello, world!", "pbkdf2-sha256$v1$5000001$c2FsdA==$aGFzaA=="))
}

func TestPasswordHashingWithPBKDF2Preferred(t *testing.T) {
	t.Parallel()

	const pass = "Hello, world!"

	pbkdf2Hash, err := HashPassword(pass)
	require.NoError(t, err)

	valid, stale := VerifyPassword(pass, pbkdf2Hash)
	assert.True(t, valid)
	assert.False(t, stale)

	bcryptHash, err := (BcryptPasswordHasher{}).HashPassword(pass)
	require.NoError(t, err)

	valid, stale = VerifyPassword(pass, bcryptHash)
	assert.True(t, valid)
	assert.True(t, stale)
}

func TestDummyPasswordHasher(t *testing.T) {
	t.Parallel()
	h := DummyPasswordHasher{}
	testPasswordHasher(t, h)
}

func TestPasswordHashing(t *testing.T) {
	t.Parallel()
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
