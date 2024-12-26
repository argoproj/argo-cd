package git

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	gocache "github.com/patrickmn/go-cache"

	argoio "github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	"github.com/bradleyfalzon/ghinstallation/v2"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/common"
	certutil "github.com/argoproj/argo-cd/v2/util/cert"
	argoioutils "github.com/argoproj/argo-cd/v2/util/io"
)

var (
	// In memory cache for storing github APP api token credentials
	githubAppTokenCache *gocache.Cache
	// In memory cache for storing oauth2.TokenSource used to generate Google Cloud OAuth tokens
	googleCloudTokenSource *gocache.Cache
)

const (
	// githubAccessTokenUsername is a username that is used to with the github access token
	githubAccessTokenUsername = "x-access-token"
	forceBasicAuthHeaderEnv   = "ARGOCD_GIT_AUTH_HEADER"
)

func init() {
	githubAppCredsExp := common.GithubAppCredsExpirationDuration
	if exp := os.Getenv(common.EnvGithubAppCredsExpirationDuration); exp != "" {
		if qps, err := strconv.Atoi(exp); err != nil {
			githubAppCredsExp = time.Duration(qps) * time.Minute
		}
	}

	githubAppTokenCache = gocache.New(githubAppCredsExp, 1*time.Minute)
	// oauth2.TokenSource handles fetching new Tokens once they are expired. The oauth2.TokenSource itself does not expire.
	googleCloudTokenSource = gocache.New(gocache.NoExpiration, 0)
}

type NoopCredsStore struct{}

func (d NoopCredsStore) Add(username string, password string) string {
	return ""
}

func (d NoopCredsStore) Remove(id string) {
}

func (d NoopCredsStore) Environ(id string) []string {
	return []string{}
}

type CredsStore interface {
	Add(username string, password string) string
	Remove(id string)
	// Environ returns the environment variables that should be set to use the credentials for the given credential ID.
	Environ(id string) []string
}

type Creds interface {
	Environ() (io.Closer, []string, error)
	// GetUserInfo gets the username and email address for the credentials, if they're available.
	GetUserInfo(ctx context.Context) (string, string, error)
}

// nop implementation
type NopCloser struct{}

func (c NopCloser) Close() error {
	return nil
}

var _ Creds = NopCreds{}

type NopCreds struct{}

func (c NopCreds) Environ() (io.Closer, []string, error) {
	return NopCloser{}, nil, nil
}

// GetUserInfo returns empty strings for user info
func (c NopCreds) GetUserInfo(ctx context.Context) (name string, email string, err error) {
	return "", "", nil
}

var _ io.Closer = NopCloser{}

type GenericHTTPSCreds interface {
	HasClientCert() bool
	GetClientCertData() string
	GetClientCertKey() string
	Creds
}

var (
	_ GenericHTTPSCreds = HTTPSCreds{}
	_ Creds             = HTTPSCreds{}
)

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
	// HTTP/HTTPS proxy used to access repository
	proxy string
	// list of targets that shouldn't use the proxy, applies only if the proxy is set
	noProxy string
	// temporal credentials store
	store CredsStore
	// whether to force usage of basic auth
	forceBasicAuth bool
}

func NewHTTPSCreds(username string, password string, clientCertData string, clientCertKey string, insecure bool, proxy string, noProxy string, store CredsStore, forceBasicAuth bool) GenericHTTPSCreds {
	return HTTPSCreds{
		username,
		password,
		insecure,
		clientCertData,
		clientCertKey,
		proxy,
		noProxy,
		store,
		forceBasicAuth,
	}
}

// GetUserInfo returns the username and email address for the credentials, if they're available.
func (c HTTPSCreds) GetUserInfo(ctx context.Context) (string, string, error) {
	// Email not implemented for HTTPS creds.
	return c.username, "", nil
}

func (c HTTPSCreds) BasicAuthHeader() string {
	h := "Authorization: Basic "
	t := c.username + ":" + c.password
	h += base64.StdEncoding.EncodeToString([]byte(t))
	return h
}

// Get additional required environment variables for executing git client to
// access specific repository via HTTPS.
func (c HTTPSCreds) Environ() (io.Closer, []string, error) {
	var env []string

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
		certFile, err := os.CreateTemp(argoio.TempDir, "")
		if err == nil {
			defer certFile.Close()
			keyFile, err = os.CreateTemp(argoio.TempDir, "")
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
		env = append(env, "GIT_SSL_CERT="+certFile.Name())

		_, err = keyFile.WriteString(c.clientCertKey)
		if err != nil {
			httpCloser.Close()
			return NopCloser{}, nil, err
		}
		// GIT_SSL_KEY is the full path to a client certificate's key to be used
		env = append(env, "GIT_SSL_KEY="+keyFile.Name())
	}
	// If at least password is set, we will set ARGOCD_BASIC_AUTH_HEADER to
	// hold the HTTP authorization header, so auth mechanism negotiation is
	// skipped. This is insecure, but some environments may need it.
	if c.password != "" && c.forceBasicAuth {
		env = append(env, fmt.Sprintf("%s=%s", forceBasicAuthHeaderEnv, c.BasicAuthHeader()))
	}
	nonce := c.store.Add(text.FirstNonEmpty(c.username, githubAccessTokenUsername), c.password)
	env = append(env, c.store.Environ(nonce)...)
	return argoioutils.NewCloser(func() error {
		c.store.Remove(nonce)
		return httpCloser.Close()
	}), env, nil
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

var _ Creds = SSHCreds{}

// SSH implementation
type SSHCreds struct {
	sshPrivateKey string
	caPath        string
	insecure      bool
	store         CredsStore
	proxy         string
	noProxy       string
}

func NewSSHCreds(sshPrivateKey string, caPath string, insecureIgnoreHostKey bool, store CredsStore, proxy string, noProxy string) SSHCreds {
	return SSHCreds{sshPrivateKey, caPath, insecureIgnoreHostKey, store, proxy, noProxy}
}

// GetUserInfo returns empty strings for user info.
// TODO: Implement this method to return the username and email address for the credentials, if they're available.
func (c SSHCreds) GetUserInfo(ctx context.Context) (string, string, error) {
	// User info not implemented for SSH creds.
	return "", "", nil
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
	file, err := os.CreateTemp(argoio.TempDir, "")
	if err != nil {
		return nil, nil, err
	}

	sshCloser := sshPrivateKeyFile(file.Name())

	defer func() {
		if err = file.Close(); err != nil {
			log.WithFields(log.Fields{
				common.SecurityField:    common.SecurityMedium,
				common.SecurityCWEField: common.SecurityCWEMissingReleaseOfFileDescriptor,
			}).Errorf("error closing file %q: %v", file.Name(), err)
		}
	}()

	_, err = file.WriteString(c.sshPrivateKey + "\n")
	if err != nil {
		sshCloser.Close()
		return nil, nil, err
	}

	args := []string{"ssh", "-i", file.Name()}
	var env []string
	if c.caPath != "" {
		env = append(env, "GIT_SSL_CAINFO="+c.caPath)
	}
	if c.insecure {
		log.Warn("temporarily disabling strict host key checking (i.e. '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'), please don't use in production")
		// StrictHostKeyChecking will add the host to the knownhosts file,  we don't want that - a security issue really,
		// UserKnownHostsFile=/dev/null is therefore used so we write the new insecure host to /dev/null
		args = append(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	} else {
		knownHostsFile := certutil.GetSSHKnownHostsDataPath()
		args = append(args, "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile="+knownHostsFile)
	}
	// Handle SSH socks5 proxy settings
	proxyEnv := []string{}
	if c.proxy != "" {
		parsedProxyURL, err := url.Parse(c.proxy)
		if err != nil {
			sshCloser.Close()
			return nil, nil, fmt.Errorf("failed to set environment variables related to socks5 proxy, could not parse proxy URL '%s': %w", c.proxy, err)
		}
		args = append(args, "-o", fmt.Sprintf("ProxyCommand='connect-proxy -S %s:%s -5 %%h %%p'",
			parsedProxyURL.Hostname(),
			parsedProxyURL.Port()))
		if parsedProxyURL.User != nil {
			proxyEnv = append(proxyEnv, "SOCKS5_USER="+parsedProxyURL.User.Username())
			if socks5_passwd, isPasswdSet := parsedProxyURL.User.Password(); isPasswdSet {
				proxyEnv = append(proxyEnv, "SOCKS5_PASSWD="+socks5_passwd)
			}
		}
	}
	env = append(env, []string{"GIT_SSH_COMMAND=" + strings.Join(args, " ")}...)
	env = append(env, proxyEnv...)
	return sshCloser, env, nil
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
	proxy          string
	noProxy        string
	store          CredsStore
}

// NewGitHubAppCreds provide github app credentials
func NewGitHubAppCreds(appID int64, appInstallId int64, privateKey string, baseURL string, repoURL string, clientCertData string, clientCertKey string, insecure bool, proxy string, noProxy string, store CredsStore) GenericHTTPSCreds {
	return GitHubAppCreds{appID: appID, appInstallId: appInstallId, privateKey: privateKey, baseURL: baseURL, repoURL: repoURL, clientCertData: clientCertData, clientCertKey: clientCertKey, insecure: insecure, proxy: proxy, noProxy: noProxy, store: store}
}

func (g GitHubAppCreds) Environ() (io.Closer, []string, error) {
	token, err := g.getAccessToken()
	if err != nil {
		return NopCloser{}, nil, err
	}
	var env []string
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
		certFile, err := os.CreateTemp(argoio.TempDir, "")
		if err == nil {
			defer certFile.Close()
			keyFile, err = os.CreateTemp(argoio.TempDir, "")
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
		env = append(env, "GIT_SSL_CERT="+certFile.Name())

		_, err = keyFile.WriteString(g.clientCertKey)
		if err != nil {
			httpCloser.Close()
			return NopCloser{}, nil, err
		}
		// GIT_SSL_KEY is the full path to a client certificate's key to be used
		env = append(env, "GIT_SSL_KEY="+keyFile.Name())
	}
	nonce := g.store.Add(githubAccessTokenUsername, token)
	env = append(env, g.store.Environ(nonce)...)
	return argoioutils.NewCloser(func() error {
		g.store.Remove(nonce)
		return httpCloser.Close()
	}), env, nil
}

// GetUserInfo returns the username and email address for the credentials, if they're available.
func (g GitHubAppCreds) GetUserInfo(ctx context.Context) (string, string, error) {
	// We use the apps transport to get the app slug.
	appTransport, err := g.getAppTransport()
	if err != nil {
		return "", "", fmt.Errorf("failed to create GitHub app transport: %w", err)
	}
	appClient := github.NewClient(&http.Client{Transport: appTransport})
	app, _, err := appClient.Apps.Get(ctx, "")
	if err != nil {
		return "", "", fmt.Errorf("failed to get app info: %w", err)
	}

	// Then we use the installation transport to get the installation info.
	appInstallTransport, err := g.getInstallationTransport()
	if err != nil {
		return "", "", fmt.Errorf("failed to get app installation: %w", err)
	}
	httpClient := http.Client{Transport: appInstallTransport}
	client := github.NewClient(&httpClient)

	appLogin := app.GetSlug() + "[bot]"
	user, _, err := client.Users.Get(ctx, appLogin)
	if err != nil {
		return "", "", fmt.Errorf("failed to get app user info: %w", err)
	}
	authorName := user.GetLogin()
	authorEmail := fmt.Sprintf("%d+%s@users.noreply.github.com", user.GetID(), user.GetLogin())
	return authorName, authorEmail, nil
}

// getAccessToken fetches GitHub token using the app id, install id, and private key.
// the token is then cached for re-use.
func (g GitHubAppCreds) getAccessToken() (string, error) {
	// Timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	itr, err := g.getInstallationTransport()
	if err != nil {
		return "", fmt.Errorf("failed to create GitHub app installation transport: %w", err)
	}

	return itr.Token(ctx)
}

// getAppTransport creates a new GitHub transport for the app
func (g GitHubAppCreds) getAppTransport() (*ghinstallation.AppsTransport, error) {
	// GitHub API url
	baseUrl := "https://api.github.com"
	if g.baseURL != "" {
		baseUrl = strings.TrimSuffix(g.baseURL, "/")
	}

	// Create a new GitHub transport
	c := GetRepoHTTPClient(baseUrl, g.insecure, g, g.proxy, g.noProxy)
	itr, err := ghinstallation.NewAppsTransport(c.Transport,
		g.appID,
		[]byte(g.privateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub installation transport: %w", err)
	}

	itr.BaseURL = baseUrl

	return itr, nil
}

// getInstallationTransport creates a new GitHub transport for the app installation
func (g GitHubAppCreds) getInstallationTransport() (*ghinstallation.Transport, error) {
	// Compute hash of creds for lookup in cache
	h := sha256.New()
	_, err := h.Write([]byte(fmt.Sprintf("%s %d %d %s", g.privateKey, g.appID, g.appInstallId, g.baseURL)))
	if err != nil {
		return nil, fmt.Errorf("failed to get get SHA256 hash for GitHub app credentials: %w", err)
	}
	key := hex.EncodeToString(h.Sum(nil))

	// Check cache for GitHub transport which helps fetch an API token
	t, found := githubAppTokenCache.Get(key)
	if found {
		itr := t.(*ghinstallation.Transport)
		// This method caches the token and if it's expired retrieves a new one
		return itr, nil
	}

	// GitHub API url
	baseUrl := "https://api.github.com"
	if g.baseURL != "" {
		baseUrl = strings.TrimSuffix(g.baseURL, "/")
	}

	// Create a new GitHub transport
	c := GetRepoHTTPClient(baseUrl, g.insecure, g, g.proxy, g.noProxy)
	itr, err := ghinstallation.New(c.Transport,
		g.appID,
		g.appInstallId,
		[]byte(g.privateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub installation transport: %w", err)
	}

	itr.BaseURL = baseUrl

	// Add transport to cache
	githubAppTokenCache.Set(key, itr, time.Minute*60)

	return itr, nil
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

var _ Creds = GoogleCloudCreds{}

// GoogleCloudCreds to authenticate to Google Cloud Source repositories
type GoogleCloudCreds struct {
	creds *google.Credentials
	store CredsStore
}

func NewGoogleCloudCreds(jsonData string, store CredsStore) GoogleCloudCreds {
	creds, err := google.CredentialsFromJSON(context.Background(), []byte(jsonData), "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		// Invalid JSON
		log.Errorf("Failed reading credentials from JSON: %+v", err)
	}
	return GoogleCloudCreds{creds, store}
}

// GetUserInfo returns the username and email address for the credentials, if they're available.
// TODO: implement getting email instead of just username.
func (c GoogleCloudCreds) GetUserInfo(ctx context.Context) (string, string, error) {
	username, err := c.getUsername()
	if err != nil {
		return "", "", fmt.Errorf("failed to get username from creds: %w", err)
	}
	return username, "", nil
}

func (c GoogleCloudCreds) Environ() (io.Closer, []string, error) {
	username, err := c.getUsername()
	if err != nil {
		return NopCloser{}, nil, fmt.Errorf("failed to get username from creds: %w", err)
	}
	token, err := c.getAccessToken()
	if err != nil {
		return NopCloser{}, nil, fmt.Errorf("failed to get access token from creds: %w", err)
	}

	nonce := c.store.Add(username, token)
	env := c.store.Environ(nonce)

	return argoioutils.NewCloser(func() error {
		c.store.Remove(nonce)
		return NopCloser{}.Close()
	}), env, nil
}

func (c GoogleCloudCreds) getUsername() (string, error) {
	type googleCredentialsFile struct {
		Type string `json:"type"`

		// Service Account fields
		ClientEmail  string `json:"client_email"`
		PrivateKeyID string `json:"private_key_id"`
		PrivateKey   string `json:"private_key"`
		AuthURL      string `json:"auth_uri"`
		TokenURL     string `json:"token_uri"`
		ProjectID    string `json:"project_id"`
	}

	if c.creds == nil {
		return "", errors.New("credentials for Google Cloud Source repositories are invalid")
	}

	var f googleCredentialsFile
	if err := json.Unmarshal(c.creds.JSON, &f); err != nil {
		return "", fmt.Errorf("failed to unmarshal Google Cloud credentials: %w", err)
	}
	return f.ClientEmail, nil
}

func (c GoogleCloudCreds) getAccessToken() (string, error) {
	if c.creds == nil {
		return "", errors.New("credentials for Google Cloud Source repositories are invalid")
	}

	// Compute hash of creds for lookup in cache
	h := sha256.New()
	_, err := h.Write(c.creds.JSON)
	if err != nil {
		return "", err
	}
	key := hex.EncodeToString(h.Sum(nil))

	t, found := googleCloudTokenSource.Get(key)
	if found {
		ts := t.(*oauth2.TokenSource)
		token, err := (*ts).Token()
		if err != nil {
			return "", fmt.Errorf("failed to get token from Google Cloud token source: %w", err)
		}
		return token.AccessToken, nil
	}

	ts := c.creds.TokenSource

	// Add TokenSource to cache
	// As TokenSource handles refreshing tokens once they expire itself, TokenSource itself can be reused. Hence, no expiration.
	googleCloudTokenSource.Set(key, &ts, gocache.NoExpiration)

	token, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("failed to get get SHA256 hash for Google Cloud credentials: %w", err)
	}

	return token.AccessToken, nil
}
