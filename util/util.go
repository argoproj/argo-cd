package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
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

// SliceCopy generates a deep copy of a slice containing any type that implements the runtime.Object interface.
func SliceCopy[T runtime.Object](items []T) []T {
	itemsCopy := make([]T, len(items))
	for i, item := range items {
		itemsCopy[i] = item.DeepCopyObject().(T)
	}
	return itemsCopy
}

// GenerateCacheKey generates a cache key based on a format string and arguments
func GenerateCacheKey(format string, args ...any) (string, error) {
	h := sha256.New()
	_, err := fmt.Fprintf(h, format, args...)
	if err != nil {
		return "", err
	}

	key := hex.EncodeToString(h.Sum(nil))
	return key, nil
}
