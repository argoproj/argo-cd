package commands

import (
	"testing"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenExpiration(t *testing.T) {
	t.Parallel()

	t.Run("reads exp claim from a valid JWT", func(t *testing.T) {
		t.Parallel()
		exp := time.Now().Add(time.Hour).Truncate(time.Second)
		token := jwtgo.NewWithClaims(jwtgo.SigningMethodHS256, jwtgo.MapClaims{
			"exp": exp.Unix(),
		})
		signed, err := token.SignedString([]byte("test-secret"))
		require.NoError(t, err)

		assert.Equal(t, exp.Unix(), tokenExpiration(signed).Unix())
	})

	t.Run("falls back to a short TTL for a non-JWT token", func(t *testing.T) {
		t.Parallel()
		got := tokenExpiration("not-a-jwt")
		assert.WithinDuration(t, time.Now().Add(time.Minute), got, 5*time.Second)
	})

	t.Run("falls back to a short TTL when exp is missing", func(t *testing.T) {
		t.Parallel()
		token := jwtgo.NewWithClaims(jwtgo.SigningMethodHS256, jwtgo.MapClaims{
			"sub": "system:serviceaccount:argocd:argocd-application-controller",
		})
		signed, err := token.SignedString([]byte("test-secret"))
		require.NoError(t, err)

		assert.WithinDuration(t, time.Now().Add(time.Minute), tokenExpiration(signed), 5*time.Second)
	})
}
