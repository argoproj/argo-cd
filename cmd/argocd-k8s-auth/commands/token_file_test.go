package commands

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTokenFile(t *testing.T) {
	t.Run("will return the file", func(t *testing.T) {
		// given
		t.Parallel()
		expirationTime := metav1.NewTime(time.Now().Add(time.Hour).Truncate(time.Second))
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"foo": "bar",
			"exp": expirationTime.Unix(),
		})
		mySigningKey := []byte("secret")
		tokenString, err := token.SignedString(mySigningKey)
		require.NoError(t, err)
		tempFile, err := os.CreateTemp("", "token-")
		require.NoError(t, err)
		_, err = tempFile.WriteString(tokenString)
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())
		defer os.Remove(tempFile.Name())

		// when
		actual, err := getTokenFile(tempFile.Name())

		// then
		assert.NoError(t, err)
		execCredential := &clientauthv1beta1.ExecCredential{}
		assert.NoError(t, json.Unmarshal([]byte(actual), &execCredential))
		assert.Equal(t, tokenString, execCredential.Status.Token)
		assert.Equal(t, &expirationTime, execCredential.Status.ExpirationTimestamp)
	})
	t.Run("will return error on malformed file", func(t *testing.T) {
		// given
		t.Parallel()
		tempFile, err := os.CreateTemp("", "token-")
		require.NoError(t, err)
		_, err = tempFile.WriteString("bogus")
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())
		defer os.Remove(tempFile.Name())

		// when
		_, err = getTokenFile(tempFile.Name())

		// then
		assert.Error(t, err)
		assert.EqualError(t, err, "token contains an invalid number of segments")
	})
	t.Run("will return error on missing expiration", func(t *testing.T) {
		// given
		t.Parallel()
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"foo": "bar",
		})
		mySigningKey := []byte("secret")
		tokenString, err := token.SignedString(mySigningKey)
		require.NoError(t, err)
		tempFile, err := os.CreateTemp("", "token-")
		require.NoError(t, err)
		_, err = tempFile.WriteString(tokenString)
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())
		defer os.Remove(tempFile.Name())

		// when
		_, err = getTokenFile(tempFile.Name())

		// then
		assert.Error(t, err)
		assert.EqualError(t, err, "Error reading expiration date from token "+tempFile.Name())
	})
}
