package git

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"

	argoio "github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/bradleyfalzon/ghinstallation"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/common"

	certutil "github.com/argoproj/argo-cd/util/cert"
)

// In memory cache for storing github APP api token credentials
var (
	githubAppTokenCache *gocache.Cache
)

func init() {
	githubAppCredsExp := common.GithubAppCredsExpirationDuration
	if exp := os.Getenv(common.EnvGithubAppCredsExpirationDuration); exp != "" {
		if qps, err := strconv.Atoi(exp); err != nil {
			githubAppCredsExp = time.Duration(qps) * time.Minute
		}
	}

	githubAppTokenCache = gocache.New(githubAppCredsExp, 1*time.Minute)
}

type Creds interface {
	Environ() (io.Closer, []string, error)
}

// nop implementation
type NopCloser struct {
}

func (c NopCloser) Close() error {
	return nil
}

type NopCreds struct {
}

func (c NopCreds) Environ() (io.Closer, []string, error) {
	return NopCloser{}, nil, nil
}

type GenericHTTPSCreds interface {
	HasClientCert() bool
	GetClientCertData() string
	GetClientCertKey() string
	Environ() (io.Closer, []string, error)
}

// HTTPS creds implementation
type HTTPSCreds struct {
	// Username for authentication
	username string
	// Password for authentication
	password string
	// Whether to ignore invalid server certificates
	insecure bool
	// Client certificate to use
	clientCertData string
	// Client certificate key to use
	clientCertKey string
}

func NewHTTPSCreds(username string, password string, clientCertData string, clientCertKey string, insecure bool) GenericHTTPSCreds {
	return HTTPSCreds{
		username,
		password,
		insecure,
		clientCertData,
		clientCertKey,
	}
}

// Get additional required environment variables for executing git client to
// access specific repository via HTTPS.
func (c HTTPSCreds) Environ() (io.Closer, []string, error) {
	env := []string{fmt.Sprintf("GIT_ASKPASS=%s", "git-ask-pass.sh"), fmt.Sprintf("GIT_USERNAME=%s", c.username), fmt.Sprintf("GIT_PASSWORD=%s", c.password)}
	httpCloser := authFilePaths(make([]string, 0))

	// GIT_SSL_NO_VERIFY is used to tell git not to validate the server's cert at
	// all.
	if c.insecure {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}

	// In case the repo is configured for using a TLS client cert, we need to make
	// sure git client will use it. The certificate's key must not be password
	// protected.
	if c.HasClientCert() {
		var certFile, keyFile *os.File

		// We need to actually create two temp files, one for storing cert data and
		// another for storing the key. If we fail to create second fail, the first
		// must be removed.
		certFile, err := ioutil.TempFile(argoio.TempDir, "")
		if err == nil {
			defer certFile.Close()
			keyFile, err = ioutil.TempFile(argoio.TempDir, "")
			if err != nil {
				removeErr := os.Remove(certFile.Name())
				if removeErr != nil {
					log.Errorf("Could not remove previously created tempfile %s: %v", certFile.Name(), removeErr)
				}
				return NopCloser{}, nil, err
			}
			defer keyFile.Close()
		} else {
			return NopCloser{}, nil, err
		}

		// We should have both temp files by now
		httpCloser = authFilePaths([]string{certFile.Name(), keyFile.Name()})

		_, err = certFile.WriteString(c.clientCertData)
		if err != nil {
			httpCloser.Close()
			return NopCloser{}, nil, err
		}
		// GIT_SSL_CERT is the full path to a client certificate to be used
		env = append(env, fmt.Sprintf("GIT_SSL_CERT=%s", certFile.Name()))

		_, err = keyFile.WriteString(c.clientCertKey)
		if err != nil {
			httpCloser.Close()
			return NopCloser{}, nil, err
		}
		// GIT_SSL_KEY is the full path to a client certificate's key to be used
		env = append(env, fmt.Sprintf("GIT_SSL_KEY=%s", keyFile.Name()))

	}
	return httpCloser, env, nil
}

func (g HTTPSCreds) HasClientCert() bool {
	return g.clientCertData != "" && g.clientCertKey != ""
}

func (c HTTPSCreds) GetClientCertData() string {
	return c.clientCertData
}

func (c HTTPSCreds) GetClientCertKey() string {
	return c.clientCertKey
}

// SSH implementation
type SSHCreds struct {
	sshPrivateKey string
	caPath        string
	insecure      bool
}

func NewSSHCreds(sshPrivateKey string, caPath string, insecureIgnoreHostKey bool) SSHCreds {
	return SSHCreds{sshPrivateKey, caPath, insecureIgnoreHostKey}
}

type sshPrivateKeyFile string

type authFilePaths []string

func (f sshPrivateKeyFile) Close() error {
	return os.Remove(string(f))
}

// Remove a list of files that have been created as temp files while creating
// HTTPCreds object above.
func (f authFilePaths) Close() error {
	var retErr error = nil
	for _, path := range f {
		err := os.Remove(path)
		if err != nil {
			log.Errorf("HTTPSCreds.Close(): Could not remove temp file %s: %v", path, err)
			retErr = err
		}
	}
	return retErr
}

func (c SSHCreds) Environ() (io.Closer, []string, error) {
	// use the SHM temp dir from util, more secure
	file, err := ioutil.TempFile(argoio.TempDir, "")
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	_, err = file.WriteString(c.sshPrivateKey + "\n")
	if err != nil {
		return nil, nil, err
	}

	args := []string{"ssh", "-i", file.Name()}
	var env []string
	if c.caPath != "" {
		env = append(env, fmt.Sprintf("GIT_SSL_CAINFO=%s", c.caPath))
	}
	if c.insecure {
		log.Warn("temporarily disabling strict host key checking (i.e. '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'), please don't use in production")
		// StrictHostKeyChecking will add the host to the knownhosts file,  we don't want that - a security issue really,
		// UserKnownHostsFile=/dev/null is therefore used so we write the new insecure host to /dev/null
		args = append(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	} else {
		knownHostsFile := certutil.GetSSHKnownHostsDataPath()
		args = append(args, "-o", "StrictHostKeyChecking=yes", "-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsFile))
	}
	env = append(env, []string{fmt.Sprintf("GIT_SSH_COMMAND=%s", strings.Join(args, " "))}...)
	return sshPrivateKeyFile(file.Name()), env, nil
}

// GitHubAppCreds to authenticate as GitHub application
type GitHubAppCreds struct {
	appID          int64
	appInstallId   int64
	privateKey     string
	baseURL        string
	repoURL        string
	clientCertData string
	clientCertKey  string
	insecure       bool
}

// NewGitHubAppCreds provide github app credentials
func NewGitHubAppCreds(appID int64, appInstallId int64, privateKey string, baseURL string, repoURL string, clientCertData string, clientCertKey string, insecure bool) GenericHTTPSCreds {
	return GitHubAppCreds{appID: appID, appInstallId: appInstallId, privateKey: privateKey, baseURL: baseURL, repoURL: repoURL, clientCertData: clientCertData, clientCertKey: clientCertKey, insecure: insecure}
}

func (g GitHubAppCreds) Environ() (io.Closer, []string, error) {
	token, err := g.getAccessToken()
	if err != nil {
		return NopCloser{}, nil, err
	}

	env := []string{fmt.Sprintf("GIT_ASKPASS=%s", "git-ask-pass.sh"), "GIT_USERNAME=x-access-token", fmt.Sprintf("GIT_PASSWORD=%s", token)}
	httpCloser := authFilePaths(make([]string, 0))

	// GIT_SSL_NO_VERIFY is used to tell git not to validate the server's cert at
	// all.
	if g.insecure {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}

	// In case the repo is configured for using a TLS client cert, we need to make
	// sure git client will use it. The certificate's key must not be password
	// protected.
	if g.HasClientCert() {
		var certFile, keyFile *os.File

		// We need to actually create two temp files, one for storing cert data and
		// another for storing the key. If we fail to create second fail, the first
		// must be removed.
		certFile, err := ioutil.TempFile(argoio.TempDir, "")
		if err == nil {
			defer certFile.Close()
			keyFile, err = ioutil.TempFile(argoio.TempDir, "")
			if err != nil {
				removeErr := os.Remove(certFile.Name())
				if removeErr != nil {
					log.Errorf("Could not remove previously created tempfile %s: %v", certFile.Name(), removeErr)
				}
				return NopCloser{}, nil, err
			}
			defer keyFile.Close()
		} else {
			return NopCloser{}, nil, err
		}

		// We should have both temp files by now
		httpCloser = authFilePaths([]string{certFile.Name(), keyFile.Name()})

		_, err = certFile.WriteString(g.clientCertData)
		if err != nil {
			httpCloser.Close()
			return NopCloser{}, nil, err
		}
		// GIT_SSL_CERT is the full path to a client certificate to be used
		env = append(env, fmt.Sprintf("GIT_SSL_CERT=%s", certFile.Name()))

		_, err = keyFile.WriteString(g.clientCertKey)
		if err != nil {
			httpCloser.Close()
			return NopCloser{}, nil, err
		}
		// GIT_SSL_KEY is the full path to a client certificate's key to be used
		env = append(env, fmt.Sprintf("GIT_SSL_KEY=%s", keyFile.Name()))

	}
	return httpCloser, env, nil
}

// getAccessToken fetches GitHub token using the app id, install id, and private key.
// the token is then cached for re-use.
func (g GitHubAppCreds) getAccessToken() (string, error) {
	// Timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Compute hash of creds for lookup in cache
	h := sha256.New()
	_, err := h.Write([]byte(fmt.Sprintf("%s %d %d %s", g.privateKey, g.appID, g.appInstallId, g.baseURL)))
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%x", h.Sum(nil))

	// Check cache for GitHub transport which helps fetch an API token
	t, found := githubAppTokenCache.Get(key)
	if found {
		itr := t.(*ghinstallation.Transport)
		// This method caches the token and if it's expired retrieves a new one
		return itr.Token(ctx)
	}

	// GitHub API url
	baseUrl := "https://api.github.com"
	if g.baseURL != "" {
		baseUrl = strings.TrimSuffix(g.baseURL, "/")
	}

	// Create a new GitHub transport
	c := GetRepoHTTPClient(baseUrl, g.insecure, g)
	itr, err := ghinstallation.New(c.Transport,
		g.appID,
		g.appInstallId,
		[]byte(g.privateKey),
	)
	if err != nil {
		return "", err
	}

	itr.BaseURL = baseUrl

	// Add transport to cache
	githubAppTokenCache.Set(key, itr, time.Minute*60)

	return itr.Token(ctx)
}

func (g GitHubAppCreds) HasClientCert() bool {
	return g.clientCertData != "" && g.clientCertKey != ""
}

func (g GitHubAppCreds) GetClientCertData() string {
	return g.clientCertData
}

func (g GitHubAppCreds) GetClientCertKey() string {
	return g.clientCertKey
}
