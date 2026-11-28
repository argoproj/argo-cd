package password

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strconv"
	"strings"
)

const (
	// 600,000 iterations is the OWASP recommendation for PBKDF2-HMAC-SHA256.
	pbkdf2Iterations = 600000
	pbkdf2SaltLength = 16
	pbkdf2KeyLength  = 32
)

// PBKDF2PasswordHasher handles password hashing using PBKDF2-HMAC-SHA256.
type PBKDF2PasswordHasher struct{}

var _ PasswordHasher = PBKDF2PasswordHasher{}

// HashPassword hashes a password using PBKDF2-HMAC-SHA256.
func (h PBKDF2PasswordHasher) HashPassword(password string) (string, error) {
	salt := make([]byte, pbkdf2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash, err := pbkdf2.Key(
		sha256.New,
		password,
		salt,
		pbkdf2Iterations,
		pbkdf2KeyLength,
	)
	if err != nil {
		return "", err
	}

	return strings.Join([]string{
		"pbkdf2-sha256",
		"v1",
		strconv.Itoa(pbkdf2Iterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	}, "$"), nil
}

// VerifyPassword verifies a password against a PBKDF2-HMAC-SHA256 hash.
func (h PBKDF2PasswordHasher) VerifyPassword(password, hashedPassword string) bool {
	parts := strings.Split(hashedPassword, "$")
	if len(parts) != 5 || parts[0] != "pbkdf2-sha256" || parts[1] != "v1" {
		return false
	}

	iterations, err := strconv.Atoi(parts[2])
	if err != nil || iterations != pbkdf2Iterations {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(salt) != pbkdf2SaltLength {
		return false
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(expectedHash) != pbkdf2KeyLength {
		return false
	}

	actualHash, err := pbkdf2.Key(
		sha256.New,
		password,
		salt,
		iterations,
		len(expectedHash),
	)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1
}
