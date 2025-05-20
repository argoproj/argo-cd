package password

import (
	"crypto/subtle"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher is an interface type to declare a general-purpose password management tool.
type PasswordHasher interface {
	HashPassword(string) (string, error)
	VerifyPassword(string, string) bool
}

// DummyPasswordHasher is for testing ONLY.  DO NOT USE in a production context.
type DummyPasswordHasher struct{}

// BcryptPasswordHasher handles password hashing with Bcrypt.  Create with `0` as the work factor to default to bcrypt.DefaultCost at hashing time.  The Cost field represents work factor.
type BcryptPasswordHasher struct {
	Cost int
}

var (
	_ PasswordHasher = DummyPasswordHasher{}
	_ PasswordHasher = BcryptPasswordHasher{0}
)

// PreferredHashers holds the list of preferred hashing algorithms, in order of most to least preferred.  Any password that does not validate with the primary algorithm will be considered "stale."  DO NOT ADD THE DUMMY HASHER FOR USE IN PRODUCTION.
var preferredHashers = []PasswordHasher{
	BcryptPasswordHasher{},
}

// HashPasswordWithHashers hashes an entered password using the first hasher in the provided list of hashers.
func hashPasswordWithHashers(password string, hashers []PasswordHasher) (string, error) {
	// Even though good hashers will disallow blank passwords, let's be explicit that ALL BLANK PASSWORDS ARE INVALID.  Full stop.
	if password == "" {
		return "", fmt.Errorf("blank passwords are not allowed")
	}
	return hashers[0].HashPassword(password)
}

// VerifyPasswordWithHashers verifies an entered password against a hashed password using one or more algorithms.  It returns whether the hash is "stale" (i.e., was verified using something other than the FIRST hasher specified).
func verifyPasswordWithHashers(password, hashedPassword string, hashers []PasswordHasher) (bool, bool) {
	// Even though good hashers will disallow blank passwords, let's be explicit that ALL BLANK PASSWORDS ARE INVALID.  Full stop.
	if password == "" {
		return false, false
	}

	valid, stale := false, false

	for idx, hasher := range hashers {
		if hasher.VerifyPassword(password, hashedPassword) {
			valid = true
			if idx > 0 {
				stale = true
			}
			break
		}
	}

	return valid, stale
}

// HashPassword hashes against the current preferred hasher.
func HashPassword(password string) (string, error) {
	return hashPasswordWithHashers(password, preferredHashers)
}

// VerifyPassword verifies an entered password against a hashed password and returns whether the hash is "stale" (i.e., was verified using the FIRST preferred hasher above).
func VerifyPassword(password, hashedPassword string) (valid, stale bool) {
	valid, stale = verifyPasswordWithHashers(password, hashedPassword, preferredHashers)
	return
}

// HashPassword creates a one-way digest ("hash") of a password.  In the case of Bcrypt, a pseudorandom salt is included automatically by the underlying library.
func (h DummyPasswordHasher) HashPassword(password string) (string, error) {
	return password, nil
}

// VerifyPassword validates whether a one-way digest ("hash") of a password was created from a given plaintext password.
func (h DummyPasswordHasher) VerifyPassword(password, hashedPassword string) bool {
	return 1 == subtle.ConstantTimeCompare([]byte(password), []byte(hashedPassword))
}

// HashPassword creates a one-way digest ("hash") of a password.  In the case of Bcrypt, a pseudorandom salt is included automatically by the underlying library.  For security reasons, the work factor is always at _least_ bcrypt.DefaultCost.
func (h BcryptPasswordHasher) HashPassword(password string) (string, error) {
	cost := h.Cost
	if cost < bcrypt.DefaultCost {
		cost = bcrypt.DefaultCost
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		hashedPassword = []byte("")
	}
	return string(hashedPassword), err
}

// VerifyPassword validates whether a one-way digest ("hash") of a password was created from a given plaintext password.
func (h BcryptPasswordHasher) VerifyPassword(password, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}
