package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_Successful(t *testing.T) {
	encrypted, err := Encrypt([]byte("test"), "sample-password")
	require.NoError(t, err)

	decrypted, err := Decrypt(encrypted, "sample-password")
	require.NoError(t, err)

	assert.Equal(t, "test", string(decrypted))
}

func TestEncryptDecrypt_Failed(t *testing.T) {
	encrypted, err := Encrypt([]byte("test"), "sample-password")
	require.NoError(t, err)

	_, err = Decrypt(encrypted, "wrong-password")
	assert.Error(t, err)
}
