package git

import (
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

type cred struct {
	username string
	password string
}

type memoryCredsStore struct {
	creds map[string]cred
}

func (s *memoryCredsStore) Add(username string, password string) string {
	id := uuid.New().String()
	s.creds[id] = cred{
		username: username,
		password: password,
	}
	return id
}

func (s *memoryCredsStore) Remove(id string) {
	delete(s.creds, id)
}

func TestHTTPSCreds_Environ_no_cert_cleanup(t *testing.T) {
	store := &memoryCredsStore{creds: make(map[string]cred)}
	creds := NewHTTPSCreds("", "", "", "", true, "", store)
	closer, env, err := creds.Environ()
	require.NoError(t, err)
	var nonce string
	for _, envVar := range env {
		if strings.HasPrefix(envVar, ASKPASS_NONCE_ENV) {
			nonce = envVar[len(ASKPASS_NONCE_ENV) + 1:]
			break
		}
	}
	assert.Contains(t, store.creds, nonce)
	io.Close(closer)
	assert.NotContains(t, store.creds, nonce)
}

func TestHTTPSCreds_Environ_insecure_true(t *testing.T) {
	creds := NewHTTPSCreds("", "", "", "", true, "", &NoopCredsStore{})
	closer, env, err := creds.Environ()
	t.Cleanup(func() {
		io.Close(closer)
	})
	require.NoError(t, err)
	found := false
	for _, envVar := range env {
		if envVar == "GIT_SSL_NO_VERIFY=true" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestHTTPSCreds_Environ_insecure_false(t *testing.T) {
	creds := NewHTTPSCreds("", "", "", "", false, "", &NoopCredsStore{})
	closer, env, err := creds.Environ()
	t.Cleanup(func() {
		io.Close(closer)
	})
	require.NoError(t, err)
	found := false
	for _, envVar := range env {
		if envVar == "GIT_SSL_NO_VERIFY=true" {
			found = true
			break
		}
	}
	assert.False(t, found)
}

func TestHTTPSCreds_Environ_clientCert(t *testing.T) {
	store := &memoryCredsStore{creds: make(map[string]cred)}
	creds := NewHTTPSCreds("", "", "clientCertData", "clientCertKey", false, "", store)
	closer, env, err := creds.Environ()
	require.NoError(t, err)
	var cert, key string
	for _, envVar := range env {
		if strings.HasPrefix(envVar, "GIT_SSL_CERT=") {
			cert = envVar[13:]
		} else if strings.HasPrefix(envVar, "GIT_SSL_KEY=") {
			key = envVar[12:]
		}
		if cert != "" && key != "" {
			break
		}
	}
	assert.NotEmpty(t, cert)
	assert.NotEmpty(t, key)

	certBytes, err := ioutil.ReadFile(cert)
	assert.NoError(t, err)
	assert.Equal(t, "clientCertData", string(certBytes))
	keyBytes, err := ioutil.ReadFile(key)
	assert.Equal(t, "clientCertKey", string(keyBytes))
	assert.NoError(t, err)

	io.Close(closer)

	_, err = os.Stat(cert)
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(key)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
