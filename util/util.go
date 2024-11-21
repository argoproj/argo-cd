package util

import (
	"crypto/rand"
	"encoding/base64"

	apiv1 "k8s.io/api/core/v1"
)

// MakeSignature generates a cryptographically-secure pseudo-random token, based on a given number of random bytes, for signing purposes.
func MakeSignature(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		b = nil
	}
	// base64 encode it so signing key can be typed into validation utilities
	b = []byte(base64.StdEncoding.EncodeToString(b))
	return b, err
}

// SecretCopy generates a deep copy of a slice containing secrets
//
// This function takes a slice of pointers to Secrets and returns a new slice
// containing deep copies of the original secrets.
func SecretCopy(secrets []*apiv1.Secret) []*apiv1.Secret {
	secretsCopy := make([]*apiv1.Secret, len(secrets))
	for i, secret := range secrets {
		secretsCopy[i] = secret.DeepCopy()
	}
	return secretsCopy
}
