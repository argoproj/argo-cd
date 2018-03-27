package util

import (
	"testing"
)

func testPasswordHasher(t *testing.T, h PasswordHasher) {
	// Use the default work factor
	const (
		defaultPassword = "Hello, world!"
		pollution       = "extradata12345"
	)
	hashedPassword, _ := h.HashPassword(defaultPassword)
	if !h.VerifyPassword(defaultPassword, hashedPassword) {
		t.Errorf("Password \"%s\" should have validated against hash \"%s\"", defaultPassword, hashedPassword)
	}
	if h.VerifyPassword(defaultPassword, pollution+hashedPassword) {
		t.Errorf("Password \"%s\" should NOT have validated against hash \"%s\"", defaultPassword, pollution+hashedPassword)
	}
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
	)
	hashers := []PasswordHasher{
		BcryptPasswordHasher{0},
		DummyPasswordHasher{},
	}

	hashedPassword, _ := hashPasswordWithHashers(defaultPassword, hashers)
	valid, stale := verifyPasswordWithHashers(defaultPassword, hashedPassword, hashers)
	if !valid {
		t.Errorf("Password \"%s\" should have validated against hash \"%s\"", defaultPassword, hashedPassword)
	}
	if stale {
		t.Errorf("Password \"%s\" should not have been marked stale against hash \"%s\"", defaultPassword, hashedPassword)
	}
	valid, stale = verifyPasswordWithHashers(defaultPassword, defaultPassword, hashers)
	if !valid {
		t.Errorf("Password \"%s\" should have validated against itself with dummy hasher", defaultPassword)
	}
	if !stale {
		t.Errorf("Password \"%s\" should have been acknowledged stale against itself with dummy hasher", defaultPassword)
	}

}
