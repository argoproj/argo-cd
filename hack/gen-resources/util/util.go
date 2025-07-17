package util

import (
	"crypto/rand"
	"math/big"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyz123456789")

func GetRandomString() string {
	b := make([]rune, 24)
	for i := range b {
		b[i] = letters[cryptoRandSecure(int64(len(letters)))]
	}
	return string(b)
}

func cryptoRandSecure(max int64) int64 {
	nBig, _ := rand.Int(rand.Reader, big.NewInt(max))
	return nBig.Int64()
}
