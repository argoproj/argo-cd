package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newKey() ([]byte, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		b = nil
	}
	return b, err
}

func TestEncryptDecrypt_Successful(t *testing.T) {
	key, err := newKey()
	require.NoError(t, err)
	encrypted, err := Encrypt([]byte("test"), key)
	require.NoError(t, err)

	decrypted, err := Decrypt(encrypted, key)
	require.NoError(t, err)

	assert.Equal(t, "test", string(decrypted))
}

func TestEncryptDecrypt_Failed(t *testing.T) {
	key, err := newKey()
	require.NoError(t, err)
	encrypted, err := Encrypt([]byte("test"), key)
	require.NoError(t, err)

	wrongKey, err := newKey()
	require.NoError(t, err)

	_, err = Decrypt(encrypted, wrongKey)
	assert.Error(t, err)
}
