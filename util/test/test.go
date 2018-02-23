package test

import (
	"math/rand"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz"

// RandString returns a random string of a specified length
func RandString(length int) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
