package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
)

func getKey(passphrase string) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(passphrase))
	key := hasher.Sum(nil)
	return key
}

// Encrypt encrypts the given data with the given passphrase.
func Encrypt(data []byte, passphrase string) ([]byte, error) {
	block, err := aes.NewCipher(getKey(passphrase))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// Decrypt decrypts the given data using the given passphrase.
func Decrypt(data []byte, passphrase string) ([]byte, error) {
	block, err := aes.NewCipher(getKey(passphrase))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
