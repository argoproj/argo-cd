package git

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"regexp"
	"slices"
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
	found := slices.Contains(env, "GIT_SSL_NO_VERIFY=true")
	assert.True(t, found)
}

func TestHTTPSCreds_Environ_insecure_false(t *testing.T) {
	creds := NewHTTPSCreds("", "", "", "", "", false, &NoopCredsStore{}, false)
	closer, env, err := creds.Environ()
	t.Cleanup(func() {
		utilio.Close(closer)
	})
	require.NoError(t, err)
	found := slices.Contains(env, "GIT_SSL_NO_VERIFY=true")
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

	// argoio.TempDir will be /dev/shm or "" (on an OS without /dev/shm).
	// In this case os.CreateTemp(), which is used by creds.Environ(),
	// will use os.TempDir for the temporary directory.
	// Reproducing this logic here:
	argoioTempDir := argoio.TempDir
	if argoioTempDir == "" {
		argoioTempDir = os.TempDir()
	}

	// countDev returns the number of files in the temporary directory
	countFilesInDevShm := func() int {
		entries, err := os.ReadDir(argoioTempDir)
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
	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken(azureDevopsEntraResourceId).Return(&workloadidentity.Token{AccessToken: "accessToken", ExpiresOn: time.Now().Add(time.Minute)}, nil).Maybe()
	creds := AzureWorkloadIdentityCreds{store, workloadIdentityMock}
	_, env, err := creds.Environ()
	require.NoError(t, err)
	assert.Len(t, store.creds, 1)

	for _, value := range store.creds {
		assert.Empty(t, value.username)
		assert.Equal(t, "accessToken", value.password)
	}

	require.Len(t, env, 1)
	assert.Equal(t, "ARGOCD_GIT_BEARER_AUTH_HEADER=Authorization: Bearer accessToken", env[0], "ARGOCD_GIT_BEARER_AUTH_HEADER env var must be set")
}

func TestAzureWorkloadIdentityCreds_Environ_cleanup(t *testing.T) {
	resetAzureTokenCache()
	store := &memoryCredsStore{creds: make(map[string]cred)}
	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken(azureDevopsEntraResourceId).Return(&workloadidentity.Token{AccessToken: "accessToken", ExpiresOn: time.Now().Add(time.Minute)}, nil).Maybe()
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
	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken(azureDevopsEntraResourceId).Return(&workloadidentity.Token{AccessToken: "accessToken", ExpiresOn: time.Now().Add(time.Minute)}, nil).Maybe()
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
	workloadIdentityMock := &mocks.TokenProvider{}
	workloadIdentityMock.EXPECT().GetToken(azureDevopsEntraResourceId).
		Return(&workloadidentity.Token{AccessToken: "firstToken", ExpiresOn: time.Now().Add(time.Minute)}, nil).Once()
	workloadIdentityMock.EXPECT().GetToken(azureDevopsEntraResourceId).
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
	workloadIdentityMock := &mocks.TokenProvider{}
	firstToken := &workloadidentity.Token{AccessToken: "firstToken", ExpiresOn: time.Now().Add(6 * time.Minute)}
	secondToken := &workloadidentity.Token{AccessToken: "secondToken"}
	workloadIdentityMock.EXPECT().GetToken(azureDevopsEntraResourceId).Return(firstToken, nil).Once()
	workloadIdentityMock.EXPECT().GetToken(azureDevopsEntraResourceId).Return(secondToken, nil).Once()
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

const fakeGitHubAppPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEA2KHkp2fe0ReHqAt9BimWEec2ryWZIyg9jvB3BdP3mzFf0bOt
WlHm1FAETFxH4h5jYASUaaWEwRNNyGlT1GhTp+jOMC4xhOSb5/SnI2dt2EITkudQ
FKsSFUdJAndqOzkjrP2pL4fi4b7JWhuLDO36ufAP4l2m3tnAseGSSTIccWvzLFFU
s3wsHOHxOcJGCP1Z7rizxl6mTKYL/Z+GHqN17OJslDf901uPXsUeDCYL2iigGPhD
Ao6k8POsfbpqLG7poCDTK50FLnS5qEocjxt+J4ZjBEWTU/DOFXWYstzfbhm8OZPQ
pSSEiBCxpg+zjtkQfCyZxXB5RQ84CY78fXOI9QIDAQABAoIBAG8jL0FLIp62qZvm
uO9ualUo/37/lP7aaCpq50UQJ9lwjS3yNh8+IWQO4QWj2iUBXg4mi1Vf2ymKk78b
eixgkXp1D0Lcj/8ToYBwnUami04FKDGXhhf0Y8SS27vuM4vKlqjrQd7modkangYi
V0X82UKHDD8fuLpfkGIxzXDLypfMzjMuVpSntnWaf2YX3VR/0/66yEp9GejftF2k
wqhGoWM6r68pN5XuCqWd5PRluSoDy/o4BAFMhYCSfp9PjgZE8aoeWHgYzlZ3gUyn
r+HaDDNWbibhobXk/9h8lwAJ6KCZ5RZ+HFfh0HuwIxmocT9OCFgy/S0g1p+o3m9K
VNd5AMkCgYEA5fbS5UK7FBzuLoLgr1hktmbLJhpt8y8IPHNABHcUdE+O4/1xTQNf
pMUwkKjGG1MtrGjLOIoMGURKKn8lR1GMZueOTSKY0+mAWUGvSzl6vwtJwvJruT8M
otEO03o0tPnRKGxbFjqxkp2b6iqJ8MxCRZ3lSidc4mdi7PHzv9lwgvsCgYEA8Siq
7weCri9N6y+tIdORAXgRzcW54BmJyqB147c72RvbMacb6rN28KXpM3qnRXyp3Llb
yh81TW3FH10GqrjATws7BK8lP9kkAw0Z/7kNiS1NgH3pUbO+5H2kAa/6QW35nzRe
Jw2lyfYGWqYO4hYXH14ML1kjgS1hgd3XHOQ64M8CgYAKcjDYSzS2UC4dnMJaFLjW
dErsGy09a7iDDnUs/r/GHMsP3jZkWi/hCzgOiiwdl6SufUAl/FdaWnjH/2iRGco3
7nLPXC/3CFdVNp+g2iaSQRADtAFis9N+HeL/hkCYq/RtUqa8lsP0NgacF3yWnKCy
Ct8chDc67ZlXzBHXeCgdOwKBgHHGFPbWXUHeUW1+vbiyvrupsQSanznp8oclMtkv
Dk48hSokw9fzuU6Jh77gw9/Vk7HtxS9Tj+squZA1bDrJFPl1u+9WzkUUJZhG6xgp
bwhj1iejv5rrKUlVOTYOlwudXeJNa4oTNz9UEeVcaLMjZt9GmIsSC90a0uDZD26z
AlAjAoGAEoqm2DcNN7SrH6aVFzj1EVOrNsHYiXj/yefspeiEmf27PSAslP+uF820
SDpz4h+Bov5qTKkzcxuu1QWtA4M0K8Iy6IYLwb83DZEm1OsAf4i0pODz21PY/I+O
VHzjB10oYgaInHZgMUdyb6F571UdiYSB6a/IlZ3ngj5touy3VIM=
-----END RSA PRIVATE KEY-----`

func TestDiscoverGitHubAppInstallationID(t *testing.T) {
	t.Run("returns cached installation ID", func(t *testing.T) {
		// Setup: prepopulate cache
		org := "test-org"
		appId := int64(12345)
		domain := "github.com"
		expectedId := int64(98765)

		// Clean up at both start and end to ensure test isolation
		cacheKey := fmt.Sprintf("%s:%s:%d", strings.ToLower(org), domain, appId)

		githubInstallationIdCache.Set(cacheKey, expectedId, gocache.NoExpiration)

		// Ensure cleanup even if test fails
		t.Cleanup(func() {
			githubInstallationIdCache.Delete(cacheKey)
		})

		// Execute
		ctx := context.Background()
		actualId, err := DiscoverGitHubAppInstallationID(ctx, appId, "fake-key", "", org)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedId, actualId)
	})

	t.Run("discovers installation ID from GitHub API", func(t *testing.T) {
		// Setup: mock GitHub API server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// GitHub Enterprise expects paths like /api/v3/app/installations
			// go-github's WithEnterpriseURLs adds this prefix automatically
			if strings.HasSuffix(r.URL.Path, "/app/installations") {
				w.WriteHeader(http.StatusOK)
				//nolint:errcheck
				json.NewEncoder(w).Encode([]map[string]any{
					{"id": 98765, "account": map[string]any{"login": "test-org"}},
				})
				return
			}
			// Return 404 for any other path
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		// Clean up cache entry for this test on completion
		t.Cleanup(func() {
			// Extract domain from server URL for proper cache key
			domain, _ := domainFromBaseURL(server.URL)
			cacheKey := fmt.Sprintf("%s:%s:%d", strings.ToLower("test-org"), domain, 12345)
			githubInstallationIdCache.Delete(cacheKey)
		})

		// Execute & Assert
		ctx := context.Background()
		// Pass the mock server URL as the enterpriseBaseURL so the GitHub client uses it
		// Note: The mock server will have a different domain (e.g., 127.0.0.1) than the first test (github.com),
		// so there's no cache collision between the two subtests.
		actualId, err := DiscoverGitHubAppInstallationID(ctx, 12345, fakeGitHubAppPrivateKey, server.URL, "test-org")
		require.NoError(t, err)
		assert.Equal(t, int64(98765), actualId)
	})
}

func TestExtractOrgFromRepoURL(t *testing.T) {
	tests := []struct {
		name        string
		repoURL     string
		expected    string
		expectError bool
	}{
		{"HTTPS URL", "https://github.com/argoproj/argo-cd", "argoproj", false},
		{"HTTPS URL with .git", "https://github.com/argoproj/argo-cd.git", "argoproj", false},
		{"HTTPS URL with port", "https://github.com:443/argoproj/argo-cd.git", "argoproj", false},
		{"SSH URL", "git@github.com:argoproj/argo-cd.git", "argoproj", false},
		{"SSH URL without .git", "git@github.com:argoproj/argo-cd", "argoproj", false},
		{"SSH URL with ssh:// prefix", "ssh://git@github.com:argoproj/argo-cd.git", "argoproj", false},
		{"SSH URL with port", "ssh://git@github.com:22/argoproj/argo-cd.git", "argoproj", false},
		{"GitHub Enterprise HTTPS", "https://github.example.com/myorg/myrepo.git", "myorg", false},
		{"GitHub Enterprise SSH", "git@github.example.com:myorg/myrepo.git", "myorg", false},
		{"Case insensitive", "https://github.com/ArgoPROJ/argo-cd", "argoproj", false}, // Test case sensitivity
		{"Invalid URL", "not-a-url", "", true},
		{"Empty string", "", "", true},
		{"URL without org/repo", "https://github.com", "", true},
		{"URL with only org", "https://github.com/argoproj", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ExtractOrgFromRepoURL(tt.repoURL)
			if tt.expectError {
				require.Error(t, err)
				assert.Empty(t, actual)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}
