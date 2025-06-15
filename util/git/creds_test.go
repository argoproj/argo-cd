package git

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	argoio "github.com/argoproj/gitops-engine/pkg/utils/io"

	"github.com/argoproj/argo-cd/v3/util/cert"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/mocks"
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

func (s *memoryCredsStore) Environ(_ string) []string {
	return nil
}

func TestHTTPSCreds_Environ_no_cert_cleanup(t *testing.T) {
	store := &memoryCredsStore{creds: make(map[string]cred)}
	creds := NewHTTPSCreds("", "", "", "", "", true, store, false)
	closer, _, err := creds.Environ()
	require.NoError(t, err)
	credsLenBefore := len(store.creds)
	utilio.Close(closer)
	assert.Len(t, store.creds, credsLenBefore-1)
}

func TestHTTPSCreds_Environ_insecure_true(t *testing.T) {
	creds := NewHTTPSCreds("", "", "", "", "", true, &NoopCredsStore{}, false)
	closer, env, err := creds.Environ()
	t.Cleanup(func() {
		utilio.Close(closer)
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
	creds := NewHTTPSCreds("", "", "", "", "", false, &NoopCredsStore{}, false)
	closer, env, err := creds.Environ()
	t.Cleanup(func() {
		utilio.Close(closer)
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
		creds := NewHTTPSCreds("username", "password", "", "", "", false, store, true)
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		defer closer.Close()
		var header string
		for _, envVar := range env {
			if strings.HasPrefix(envVar, forceBasicAuthHeaderEnv+"=") {
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
		creds := NewHTTPSCreds("", "", "", "", "", false, store, true)
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		defer closer.Close()
		var header string
		for _, envVar := range env {
			if strings.HasPrefix(envVar, forceBasicAuthHeaderEnv+"=") {
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
		creds := NewHTTPSCreds("username", "password", "", "", "", false, store, false)
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		defer closer.Close()
		var header string
		for _, envVar := range env {
			if strings.HasPrefix(envVar, forceBasicAuthHeaderEnv+"=") {
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
		creds := NewHTTPSCreds("", "", "", "", "", false, store, false)
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		defer closer.Close()
		var header string
		for _, envVar := range env {
			if strings.HasPrefix(envVar, forceBasicAuthHeaderEnv+"=") {
				header = envVar[len(forceBasicAuthHeaderEnv)+1:]
			}
			if header != "" {
				break
			}
		}
		assert.Empty(t, header)
	})
}

func TestHTTPSCreds_Environ_bearerTokenAuth(t *testing.T) {
	t.Run("Enabled and credentials set", func(t *testing.T) {
		store := &memoryCredsStore{creds: make(map[string]cred)}
		creds := NewHTTPSCreds("", "", "token", "", "", false, store, false)
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		defer closer.Close()
		var header string
		for _, envVar := range env {
			if strings.HasPrefix(envVar, bearerAuthHeaderEnv+"=") {
				header = envVar[len(bearerAuthHeaderEnv)+1:]
			}
			if header != "" {
				break
			}
		}
		assert.Equal(t, "Authorization: Bearer token", header)
	})
}

func TestHTTPSCreds_Environ_clientCert(t *testing.T) {
	store := &memoryCredsStore{creds: make(map[string]cred)}
	creds := NewHTTPSCreds("", "", "", "clientCertData", "clientCertKey", false, store, false)
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
	require.NoError(t, err)
	assert.Equal(t, "clientCertData", string(certBytes))
	keyBytes, err := os.ReadFile(key)
	assert.Equal(t, "clientCertKey", string(keyBytes))
	require.NoError(t, err)

	utilio.Close(closer)

	_, err = os.Stat(cert)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(key)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func Test_SSHCreds_Environ(t *testing.T) {
	for _, insecureIgnoreHostKey := range []bool{false, true} {
		tempDir := t.TempDir()
		caFile := path.Join(tempDir, "caFile")
		err := os.WriteFile(caFile, []byte(""), os.FileMode(0o600))
		require.NoError(t, err)
		creds := NewSSHCreds("sshPrivateKey", caFile, insecureIgnoreHostKey, "")
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
			assert.Contains(t, env[1], "-o UserKnownHostsFile="+hostsPath)
		}

		envRegex := regexp.MustCompile("-i ([^ ]+)")
		assert.Regexp(t, envRegex, env[1])
		privateKeyFile := envRegex.FindStringSubmatch(env[1])[1]
		assert.FileExists(t, privateKeyFile)
		utilio.Close(closer)
		assert.NoFileExists(t, privateKeyFile)
	}
}

func Test_SSHCreds_Environ_WithProxy(t *testing.T) {
	for _, insecureIgnoreHostKey := range []bool{false, true} {
		tempDir := t.TempDir()
		caFile := path.Join(tempDir, "caFile")
		err := os.WriteFile(caFile, []byte(""), os.FileMode(0o600))
		require.NoError(t, err)
		creds := NewSSHCreds("sshPrivateKey", caFile, insecureIgnoreHostKey, "socks5://127.0.0.1:1080")
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
			assert.Contains(t, env[1], "-o UserKnownHostsFile="+hostsPath)
		}
		assert.Contains(t, env[1], "-o ProxyCommand='connect-proxy -S 127.0.0.1:1080 -5 %h %p'")

		envRegex := regexp.MustCompile("-i ([^ ]+)")
		assert.Regexp(t, envRegex, env[1])
		privateKeyFile := envRegex.FindStringSubmatch(env[1])[1]
		assert.FileExists(t, privateKeyFile)
		utilio.Close(closer)
		assert.NoFileExists(t, privateKeyFile)
	}
}

func Test_SSHCreds_Environ_WithProxyUserNamePassword(t *testing.T) {
	for _, insecureIgnoreHostKey := range []bool{false, true} {
		tempDir := t.TempDir()
		caFile := path.Join(tempDir, "caFile")
		err := os.WriteFile(caFile, []byte(""), os.FileMode(0o600))
		require.NoError(t, err)
		creds := NewSSHCreds("sshPrivateKey", caFile, insecureIgnoreHostKey, "socks5://user:password@127.0.0.1:1080")
		closer, env, err := creds.Environ()
		require.NoError(t, err)
		require.Len(t, env, 4)

		assert.Equal(t, fmt.Sprintf("GIT_SSL_CAINFO=%s/caFile", tempDir), env[0], "CAINFO env var must be set")

		assert.True(t, strings.HasPrefix(env[1], "GIT_SSH_COMMAND="))
		assert.Equal(t, "SOCKS5_USER=user", env[2], "SOCKS5 user env var must be set")
		assert.Equal(t, "SOCKS5_PASSWD=password", env[3], "SOCKS5 password env var must be set")

		if insecureIgnoreHostKey {
			assert.Contains(t, env[1], "-o StrictHostKeyChecking=no")
			assert.Contains(t, env[1], "-o UserKnownHostsFile=/dev/null")
		} else {
			assert.Contains(t, env[1], "-o StrictHostKeyChecking=yes")
			hostsPath := cert.GetSSHKnownHostsDataPath()
			assert.Contains(t, env[1], "-o UserKnownHostsFile="+hostsPath)
		}
		assert.Contains(t, env[1], "-o ProxyCommand='connect-proxy -S 127.0.0.1:1080 -5 %h %p'")

		envRegex := regexp.MustCompile("-i ([^ ]+)")
		assert.Regexp(t, envRegex, env[1])
		privateKeyFile := envRegex.FindStringSubmatch(env[1])[1]
		assert.FileExists(t, privateKeyFile)
		utilio.Close(closer)
		assert.NoFileExists(t, privateKeyFile)
	}
}

func Test_SSHCreds_Environ_TempFileCleanupOnInvalidProxyURL(t *testing.T) {
	// Previously, if the proxy URL was invalid, a temporary file would be left in /dev/shm. This ensures the file is cleaned up in this case.

	// countDev returns the number of files in /dev/shm (argoutilio.TempDir)
	countFilesInDevShm := func() int {
		entries, err := os.ReadDir(argoio.TempDir)
		require.NoError(t, err)

		return len(entries)
	}

	for _, insecureIgnoreHostKey := range []bool{false, true} {
		tempDir := t.TempDir()
		caFile := path.Join(tempDir, "caFile")
		err := os.WriteFile(caFile, []byte(""), os.FileMode(0o600))
		require.NoError(t, err)
		creds := NewSSHCreds("sshPrivateKey", caFile, insecureIgnoreHostKey, ":invalid-proxy-url")

		filesInDevShmBeforeInvocation := countFilesInDevShm()

		_, _, err = creds.Environ()
		require.Error(t, err)

		filesInDevShmAfterInvocation := countFilesInDevShm()

		assert.Equal(t, filesInDevShmBeforeInvocation, filesInDevShmAfterInvocation, "no temporary files should leak if the proxy url cannot be parsed")
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
	assert.Empty(t, token)
	require.Error(t, err)

	username, err := googleCloudCreds.getUsername()
	assert.Empty(t, username)
	require.Error(t, err)

	closer, envStringSlice, err := googleCloudCreds.Environ()
	assert.Equal(t, NopCloser{}, closer)
	assert.Equal(t, []string(nil), envStringSlice)
	require.Error(t, err)
}

func TestGoogleCloudCreds_Environ_cleanup(t *testing.T) {
	store := &memoryCredsStore{creds: make(map[string]cred)}
	staticToken := &oauth2.Token{AccessToken: "token"}
	googleCloudCreds := GoogleCloudCreds{&google.Credentials{
		ProjectID:   "my-google-project",
		TokenSource: oauth2.StaticTokenSource(staticToken),
		JSON:        []byte(gcpServiceAccountKeyJSON),
	}, store}

	closer, _, err := googleCloudCreds.Environ()
	require.NoError(t, err)
	credsLenBefore := len(store.creds)
	utilio.Close(closer)
	assert.Len(t, store.creds, credsLenBefore-1)
}

func TestAzureWorkloadIdentityCreds_Environ(t *testing.T) {
	resetAzureTokenCache()
	store := &memoryCredsStore{creds: make(map[string]cred)}
	workloadIdentityMock := new(mocks.TokenProvider)
	workloadIdentityMock.On("GetToken", azureDevopsEntraResourceId).Return(&workloadidentity.Token{AccessToken: "accessToken", ExpiresOn: time.Now().Add(time.Minute)}, nil)
	creds := AzureWorkloadIdentityCreds{store, workloadIdentityMock}
	_, _, err := creds.Environ()
	require.NoError(t, err)
	assert.Len(t, store.creds, 1)

	for _, value := range store.creds {
		assert.Empty(t, value.username)
		assert.Equal(t, "accessToken", value.password)
	}
}

func TestAzureWorkloadIdentityCreds_Environ_cleanup(t *testing.T) {
	resetAzureTokenCache()
	store := &memoryCredsStore{creds: make(map[string]cred)}
	workloadIdentityMock := new(mocks.TokenProvider)
	workloadIdentityMock.On("GetToken", azureDevopsEntraResourceId).Return(&workloadidentity.Token{AccessToken: "accessToken", ExpiresOn: time.Now().Add(time.Minute)}, nil)
	creds := AzureWorkloadIdentityCreds{store, workloadIdentityMock}
	closer, _, err := creds.Environ()
	require.NoError(t, err)
	credsLenBefore := len(store.creds)
	utilio.Close(closer)
	assert.Len(t, store.creds, credsLenBefore-1)
}

func TestAzureWorkloadIdentityCreds_GetUserInfo(t *testing.T) {
	resetAzureTokenCache()
	store := &memoryCredsStore{creds: make(map[string]cred)}
	workloadIdentityMock := new(mocks.TokenProvider)
	workloadIdentityMock.On("GetToken", azureDevopsEntraResourceId).Return(&workloadidentity.Token{AccessToken: "accessToken", ExpiresOn: time.Now().Add(time.Minute)}, nil)
	creds := AzureWorkloadIdentityCreds{store, workloadIdentityMock}

	user, email, err := creds.GetUserInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, workloadidentity.EmptyGuid, user)
	assert.Empty(t, email)
}

func TestGetHelmCredsShouldReturnHelmCredsIfAzureWorkloadIdentityNotSpecified(t *testing.T) {
	var creds Creds = NewAzureWorkloadIdentityCreds(NoopCredsStore{}, new(mocks.TokenProvider))

	_, ok := creds.(AzureWorkloadIdentityCreds)
	require.Truef(t, ok, "expected HelmCreds but got %T", creds)
}

func TestAzureWorkloadIdentityCreds_FetchNewTokenIfExistingIsExpired(t *testing.T) {
	resetAzureTokenCache()
	store := &memoryCredsStore{creds: make(map[string]cred)}
	workloadIdentityMock := new(mocks.TokenProvider)
	workloadIdentityMock.On("GetToken", azureDevopsEntraResourceId).
		Return(&workloadidentity.Token{AccessToken: "firstToken", ExpiresOn: time.Now().Add(time.Minute)}, nil).Once()
	workloadIdentityMock.On("GetToken", azureDevopsEntraResourceId).
		Return(&workloadidentity.Token{AccessToken: "secondToken"}, nil).Once()
	creds := AzureWorkloadIdentityCreds{store, workloadIdentityMock}
	token, err := creds.GetAzureDevOpsAccessToken()
	require.NoError(t, err)

	assert.Equal(t, "firstToken", token)
	time.Sleep(5 * time.Second)
	token, err = creds.GetAzureDevOpsAccessToken()
	require.NoError(t, err)
	assert.Equal(t, "secondToken", token)
}

func TestAzureWorkloadIdentityCreds_ReuseTokenIfExistingIsNotExpired(t *testing.T) {
	resetAzureTokenCache()
	store := &memoryCredsStore{creds: make(map[string]cred)}
	workloadIdentityMock := new(mocks.TokenProvider)
	firstToken := &workloadidentity.Token{AccessToken: "firstToken", ExpiresOn: time.Now().Add(6 * time.Minute)}
	secondToken := &workloadidentity.Token{AccessToken: "secondToken"}
	workloadIdentityMock.On("GetToken", azureDevopsEntraResourceId).Return(firstToken, nil).Once()
	workloadIdentityMock.On("GetToken", azureDevopsEntraResourceId).Return(secondToken, nil).Once()
	creds := AzureWorkloadIdentityCreds{store, workloadIdentityMock}
	token, err := creds.GetAzureDevOpsAccessToken()
	require.NoError(t, err)

	assert.Equal(t, "firstToken", token)
	time.Sleep(5 * time.Second)
	token, err = creds.GetAzureDevOpsAccessToken()
	require.NoError(t, err)
	assert.Equal(t, "firstToken", token)
}

func resetAzureTokenCache() {
	azureTokenCache = gocache.New(gocache.NoExpiration, 0)
}
