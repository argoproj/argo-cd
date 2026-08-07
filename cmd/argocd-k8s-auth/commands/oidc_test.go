package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
)

func TestValidateTokenFile(t *testing.T) {
	t.Run("returns the flag value when set", func(t *testing.T) {
		t.Setenv("ARGOCD_OIDC_TOKEN_FILE", "/env/token")
		got, err := validateTokenFile("/flag/token")
		require.NoError(t, err)
		assert.Equal(t, "/flag/token", got)
	})

	t.Run("falls back to the ARGOCD_OIDC_TOKEN_FILE env var", func(t *testing.T) {
		t.Setenv("ARGOCD_OIDC_TOKEN_FILE", "/env/token")
		got, err := validateTokenFile("")
		require.NoError(t, err)
		assert.Equal(t, "/env/token", got)
	})

	t.Run("returns an error when neither the flag nor the env var is set", func(t *testing.T) {
		t.Setenv("ARGOCD_OIDC_TOKEN_FILE", "")
		got, err := validateTokenFile("")
		require.Error(t, err)
		assert.Empty(t, got)
	})
}

func TestNewOIDCCommand_WithTokenFile(t *testing.T) {
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	token := jwtgo.NewWithClaims(jwtgo.SigningMethodHS256, jwtgo.MapClaims{
		"exp": exp.Unix(),
	})
	signed, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)

	tokenFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte(signed+"\n"), 0o600))

	out, err := captureOutput(func() error {
		cmd := newOIDCCommand()
		cmd.SetArgs([]string{"--token-file", tokenFile})
		return cmd.Execute()
	})
	require.NoError(t, err)

	var execCred clientauthv1beta1.ExecCredential
	require.NoError(t, json.Unmarshal([]byte(out), &execCred))
	assert.Equal(t, "ExecCredential", execCred.Kind)
	require.NotNil(t, execCred.Status)
	assert.Equal(t, signed, execCred.Status.Token)
	require.NotNil(t, execCred.Status.ExpirationTimestamp)
	assert.Equal(t, exp.Unix(), execCred.Status.ExpirationTimestamp.Unix())
}

// captureOutput redirects os.Stdout while f runs and returns whatever was written to it.
func captureOutput(f func() error) (string, error) {
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	err = f()
	w.Close()
	if err != nil {
		os.Stdout = stdout
		return "", err
	}
	str, err := io.ReadAll(r)
	os.Stdout = stdout
	if err != nil {
		return "", err
	}
	return string(str), err
}

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
