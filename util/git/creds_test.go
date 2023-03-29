package git

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/argoproj/argo-cd/v2/util/cert"
	"github.com/argoproj/argo-cd/v2/util/io"
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
	creds := NewHTTPSCreds("", "", "", "", true, "", store, false)
	closer, env, err := creds.Environ()
	require.NoError(t, err)
	var nonce string
	for _, envVar := range env {
		if strings.HasPrefix(envVar, ASKPASS_NONCE_ENV) {
			nonce = envVar[len(ASKPASS_NONCE_ENV)+1:]
			break
		}
	}
	assert.Contains(t, store.creds, nonce)
	io.Close(closer)
	assert.NotContains(t, store.creds, nonce)
}

func TestHTTPSCreds_Environ_insecure_true(t *testing.T) {
	creds := NewHTTPSCreds("", "", "", "", true, "", &NoopCredsStore{}, false)
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
	creds := NewHTTPSCreds("", "", "", "", false, "", &NoopCredsStore{}, false)
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

func TestHTTPSCreds_Environ_forceBasicAuth(t *testing.T) {
	t.Run("Enabled and credentials set", func(t *testing.T) {
		store := &memoryCredsStore{creds: make(map[string]cred)}
		creds := NewHTTPSCreds("username", "password", "", "", false, "", store, true)
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		defer closer.Close()
		var header string
		for _, envVar := range env {
			if strings.HasPrefix(envVar, fmt.Sprintf("%s=", forceBasicAuthHeaderEnv)) {
				header = envVar[len(forceBasicAuthHeaderEnv)+1:]
			}
			if header != "" {
				break
			}
		}
		b64enc := base64.StdEncoding.EncodeToString([]byte("username:password"))
		assert.Equal(t, "Authorization: Basic "+b64enc, header)
	})
	t.Run("Enabled but credentials not set", func(t *testing.T) {
		store := &memoryCredsStore{creds: make(map[string]cred)}
		creds := NewHTTPSCreds("", "", "", "", false, "", store, true)
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		defer closer.Close()
		var header string
		for _, envVar := range env {
			if strings.HasPrefix(envVar, fmt.Sprintf("%s=", forceBasicAuthHeaderEnv)) {
				header = envVar[len(forceBasicAuthHeaderEnv)+1:]
			}
			if header != "" {
				break
			}
		}
		assert.Empty(t, header)
	})
	t.Run("Disabled with credentials set", func(t *testing.T) {
		store := &memoryCredsStore{creds: make(map[string]cred)}
		creds := NewHTTPSCreds("username", "password", "", "", false, "", store, false)
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		defer closer.Close()
		var header string
		for _, envVar := range env {
			if strings.HasPrefix(envVar, fmt.Sprintf("%s=", forceBasicAuthHeaderEnv)) {
				header = envVar[len(forceBasicAuthHeaderEnv)+1:]
			}
			if header != "" {
				break
			}
		}
		assert.Empty(t, header)
	})

	t.Run("Disabled with credentials not set", func(t *testing.T) {
		store := &memoryCredsStore{creds: make(map[string]cred)}
		creds := NewHTTPSCreds("", "", "", "", false, "", store, false)
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		defer closer.Close()
		var header string
		for _, envVar := range env {
			if strings.HasPrefix(envVar, fmt.Sprintf("%s=", forceBasicAuthHeaderEnv)) {
				header = envVar[len(forceBasicAuthHeaderEnv)+1:]
			}
			if header != "" {
				break
			}
		}
		assert.Empty(t, header)
	})
}

func TestHTTPSCreds_Environ_clientCert(t *testing.T) {
	store := &memoryCredsStore{creds: make(map[string]cred)}
	creds := NewHTTPSCreds("", "", "clientCertData", "clientCertKey", false, "", store, false)
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

	certBytes, err := os.ReadFile(cert)
	assert.NoError(t, err)
	assert.Equal(t, "clientCertData", string(certBytes))
	keyBytes, err := os.ReadFile(key)
	assert.Equal(t, "clientCertKey", string(keyBytes))
	assert.NoError(t, err)

	io.Close(closer)

	_, err = os.Stat(cert)
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(key)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func Test_SSHCreds_Environ(t *testing.T) {
	for _, insecureIgnoreHostKey := range []bool{false, true} {
		tempDir := t.TempDir()
		caFile := path.Join(tempDir, "caFile")
		err := os.WriteFile(caFile, []byte(""), os.FileMode(0600))
		require.NoError(t, err)
		creds := NewSSHCreds("sshPrivateKey", caFile, insecureIgnoreHostKey, &NoopCredsStore{})
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		require.Len(t, env, 2)

		assert.Equal(t, fmt.Sprintf("GIT_SSL_CAINFO=%s/caFile", tempDir), env[0], "CAINFO env var must be set")

		assert.True(t, strings.HasPrefix(env[1], "GIT_SSH_COMMAND="))

		if insecureIgnoreHostKey {
			assert.Contains(t, env[1], "-o StrictHostKeyChecking=no")
			assert.Contains(t, env[1], "-o UserKnownHostsFile=/dev/null")
		} else {
			assert.Contains(t, env[1], "-o StrictHostKeyChecking=yes")
			hostsPath := cert.GetSSHKnownHostsDataPath()
			assert.Contains(t, env[1], fmt.Sprintf("-o UserKnownHostsFile=%s", hostsPath))
		}

		envRegex := regexp.MustCompile("-i ([^ ]+)")
		assert.Regexp(t, envRegex, env[1])
		privateKeyFile := envRegex.FindStringSubmatch(env[1])[1]
		assert.FileExists(t, privateKeyFile)
		io.Close(closer)
		assert.NoFileExists(t, privateKeyFile)
	}
}

const gcpServiceAccountKeyJSON = `{
  "type": "service_account",
  "project_id": "my-google-project",
  "private_key_id": "REDACTED",
  "private_key": "-----BEGIN PRIVATE KEY-----\nREDACTED\n-----END PRIVATE KEY-----\n",
  "client_email": "argocd-service-account@my-google-project.iam.gserviceaccount.com",
  "client_id": "REDACTED",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/argocd-service-account%40my-google-project.iam.gserviceaccount.com"
}`

const invalidJSON = `{
  "type": "service_account",
  "project_id": "my-google-project",
`

func TestNewGoogleCloudCreds(t *testing.T) {
	store := &memoryCredsStore{creds: make(map[string]cred)}
	googleCloudCreds := NewGoogleCloudCreds(gcpServiceAccountKeyJSON, store)
	assert.NotNil(t, googleCloudCreds)
}

func TestNewGoogleCloudCreds_invalidJSON(t *testing.T) {
	store := &memoryCredsStore{creds: make(map[string]cred)}
	googleCloudCreds := NewGoogleCloudCreds(invalidJSON, store)
	assert.Nil(t, googleCloudCreds.creds)

	token, err := googleCloudCreds.getAccessToken()
	assert.Equal(t, "", token)
	assert.NotNil(t, err)

	username, err := googleCloudCreds.getUsername()
	assert.Equal(t, "", username)
	assert.NotNil(t, err)

	closer, envStringSlice, err := googleCloudCreds.Environ()
	assert.Equal(t, NopCloser{}, closer)
	assert.Equal(t, []string(nil), envStringSlice)
	assert.NotNil(t, err)
}

func TestGoogleCloudCreds_Environ_cleanup(t *testing.T) {
	store := &memoryCredsStore{creds: make(map[string]cred)}
	staticToken := &oauth2.Token{AccessToken: "token"}
	googleCloudCreds := GoogleCloudCreds{&google.Credentials{
		ProjectID:   "my-google-project",
		TokenSource: oauth2.StaticTokenSource(staticToken),
		JSON:        []byte(gcpServiceAccountKeyJSON),
	}, store}

	closer, env, err := googleCloudCreds.Environ()
	assert.NoError(t, err)
	var nonce string
	for _, envVar := range env {
		if strings.HasPrefix(envVar, ASKPASS_NONCE_ENV) {
			nonce = envVar[len(ASKPASS_NONCE_ENV)+1:]
			break
		}
	}
	assert.Contains(t, store.creds, nonce)
	io.Close(closer)
	assert.NotContains(t, store.creds, nonce)
}
