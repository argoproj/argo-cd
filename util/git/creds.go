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
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	giturls "github.com/chainguard-dev/git-urls"
	"github.com/google/go-github/v69/github"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	gocache "github.com/patrickmn/go-cache"

	argoio "github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	"github.com/bradleyfalzon/ghinstallation/v2"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
	argoutils "github.com/argoproj/argo-cd/v3/util"
	certutil "github.com/argoproj/argo-cd/v3/util/cert"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
)

var (
	// In memory cache for storing github APP api token credentials
	githubAppTokenCache *gocache.Cache
	// In memory cache for storing oauth2.TokenSource used to generate Google Cloud OAuth tokens
	googleCloudTokenSource *gocache.Cache

	// In memory cache for storing Azure tokens
	azureTokenCache *gocache.Cache

	// installationIdCache caches installation IDs for organizations to avoid redundant API calls.
	githubInstallationIdCache      *gocache.Cache
	githubInstallationIdCacheMutex sync.RWMutex // For bulk API call coordination
)

const (
	// githubAccessTokenUsername is a username that is used to with the github access token
	githubAccessTokenUsername = "x-access-token"
	forceBasicAuthHeaderEnv   = "ARGOCD_GIT_AUTH_HEADER"
	bearerAuthHeaderEnv       = "ARGOCD_GIT_BEARER_AUTH_HEADER"
	// This is the resource id of the OAuth application of Azure Devops.
	azureDevopsEntraResourceId = "499b84ac-1321-427f-aa17-267ca6975798/.default"
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
	azureTokenCache = gocache.New(gocache.NoExpiration, 0)
	githubInstallationIdCache = gocache.New(60*time.Minute, 60*time.Minute)
}

type NoopCredsStore struct{}

func (d NoopCredsStore) Add(_ string, _ string) string {
	return ""
}

func (d NoopCredsStore) Remove(_ string) {
}

func (d NoopCredsStore) Environ(_ string) []string {
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
func (c NopCreds) GetUserInfo(_ context.Context) (name string, email string, err error) {
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
	// Bearer token for authentication
	bearerToken string
	// Whether to ignore invalid server certificates
	insecure bool
	// Client certificate to use
	clientCertData string
	// Client certificate key to use
	clientCertKey string
	// temporal credentials store
	store CredsStore
	// whether to force usage of basic auth
	forceBasicAuth bool
}

func NewHTTPSCreds(username string, password string, bearerToken string, clientCertData string, clientCertKey string, insecure bool, store CredsStore, forceBasicAuth bool) GenericHTTPSCreds {
	return HTTPSCreds{
		username,
		password,
		bearerToken,
		insecure,
		clientCertData,
		clientCertKey,
		store,
		forceBasicAuth,
	}
}

// GetUserInfo returns the username and email address for the credentials, if they're available.
func (creds HTTPSCreds) GetUserInfo(_ context.Context) (string, string, error) {
	// Email not implemented for HTTPS creds.
	return creds.username, "", nil
}

func (creds HTTPSCreds) BasicAuthHeader() string {
	h := "Authorization: Basic "
	t := creds.username + ":" + creds.password
	h += base64.StdEncoding.EncodeToString([]byte(t))
	return h
}

func (creds HTTPSCreds) BearerAuthHeader() string {
	h := "Authorization: Bearer " + creds.bearerToken
	return h
}

// Get additional required environment variables for executing git client to
// access specific repository via HTTPS.
func (creds HTTPSCreds) Environ() (io.Closer, []string, error) {
	var env []string

	httpCloser := authFilePaths(make([]string, 0))

	// GIT_SSL_NO_VERIFY is used to tell git not to validate the server's cert at
	// all.
	if creds.insecure {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}

	// In case the repo is configured for using a TLS client cert, we need to make
	// sure git client will use it. The certificate's key must not be password
	// protected.
	if creds.HasClientCert() {
		var certFile, keyFile *os.File

		// We need to actually create two temp files, one for storing cert data and
		// another for storing the key. If we fail to create second fail, the first
		// must be removed.
		certFile, err := os.CreateTemp(argoio.TempDir, "")
		if err != nil {
			return NopCloser{}, nil, err
		}
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

		// We should have both temp files by now
		httpCloser = authFilePaths([]string{certFile.Name(), keyFile.Name()})

		_, err = certFile.WriteString(creds.clientCertData)
		if err != nil {
			httpCloser.Close()
			return NopCloser{}, nil, err
		}
		// GIT_SSL_CERT is the full path to a client certificate to be used
		env = append(env, "GIT_SSL_CERT="+certFile.Name())

		_, err = keyFile.WriteString(creds.clientCertKey)
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
	if creds.password != "" && creds.forceBasicAuth {
		env = append(env, fmt.Sprintf("%s=%s", forceBasicAuthHeaderEnv, creds.BasicAuthHeader()))
	} else if creds.bearerToken != "" {
		// If bearer token is set, we will set ARGOCD_BEARER_AUTH_HEADER to	hold the HTTP authorization header
		env = append(env, fmt.Sprintf("%s=%s", bearerAuthHeaderEnv, creds.BearerAuthHeader()))
	}
	nonce := creds.store.Add(text.FirstNonEmpty(creds.username, githubAccessTokenUsername), creds.password)
	env = append(env, creds.store.Environ(nonce)...)
	return utilio.NewCloser(func() error {
		creds.store.Remove(nonce)
		return httpCloser.Close()
	}), env, nil
}

func (creds HTTPSCreds) HasClientCert() bool {
	return creds.clientCertData != "" && creds.clientCertKey != ""
}

func (creds HTTPSCreds) GetClientCertData() string {
	return creds.clientCertData
}

func (creds HTTPSCreds) GetClientCertKey() string {
	return creds.clientCertKey
}

var _ Creds = SSHCreds{}

// SSH implementation
type SSHCreds struct {
	sshPrivateKey string
	caPath        string
	insecure      bool
	proxy         string
}

func NewSSHCreds(sshPrivateKey string, caPath string, insecureIgnoreHostKey bool, proxy string) SSHCreds {
	return SSHCreds{sshPrivateKey, caPath, insecureIgnoreHostKey, proxy}
}

// GetUserInfo returns empty strings for user info.
// TODO: Implement this method to return the username and email address for the credentials, if they're available.
func (c SSHCreds) GetUserInfo(_ context.Context) (string, string, error) {
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
	var retErr error
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
			if socks5Passwd, isPasswdSet := parsedProxyURL.User.Password(); isPasswdSet {
				proxyEnv = append(proxyEnv, "SOCKS5_PASSWD="+socks5Passwd)
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
	clientCertData string
	clientCertKey  string
	insecure       bool
	proxy          string
	noProxy        string
	store          CredsStore
}

// NewGitHubAppCreds provide github app credentials
func NewGitHubAppCreds(appID int64, appInstallId int64, privateKey string, baseURL string, clientCertData string, clientCertKey string, insecure bool, proxy string, noProxy string, store CredsStore) GenericHTTPSCreds {
	return GitHubAppCreds{appID: appID, appInstallId: appInstallId, privateKey: privateKey, baseURL: baseURL, clientCertData: clientCertData, clientCertKey: clientCertKey, insecure: insecure, proxy: proxy, noProxy: noProxy, store: store}
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
		if err != nil {
			return NopCloser{}, nil, err
		}
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
	return utilio.NewCloser(func() error {
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
	baseURL := "https://api.github.com"
	if g.baseURL != "" {
		baseURL = strings.TrimSuffix(g.baseURL, "/")
	}

	// Create a new GitHub transport
	c := GetRepoHTTPClient(baseURL, g.insecure, g, g.proxy, g.noProxy)
	itr, err := ghinstallation.NewAppsTransport(c.Transport,
		g.appID,
		[]byte(g.privateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub installation transport: %w", err)
	}

	itr.BaseURL = baseURL

	return itr, nil
}

// getInstallationTransport creates a new GitHub transport for the app installation
func (g GitHubAppCreds) getInstallationTransport() (*ghinstallation.Transport, error) {
	// Compute hash of creds for lookup in cache
	h := sha256.New()
	_, err := fmt.Fprintf(h, "%s %d %d %s", g.privateKey, g.appID, g.appInstallId, g.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get SHA256 hash for GitHub app credentials: %w", err)
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
	baseURL := "https://api.github.com"
	if g.baseURL != "" {
		baseURL = strings.TrimSuffix(g.baseURL, "/")
	}

	// Create a new GitHub transport
	c := GetRepoHTTPClient(baseURL, g.insecure, g, g.proxy, g.noProxy)
	itr, err := ghinstallation.New(c.Transport,
		g.appID,
		g.appInstallId,
		[]byte(g.privateKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub installation transport: %w", err)
	}

	itr.BaseURL = baseURL

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

// GitHub App installation discovery cache and helper

// DiscoverGitHubAppInstallationID discovers the GitHub App installation ID for a given organization.
// It queries the GitHub API to list all installations for the app and returns the installation ID
// for the matching organization. Results are cached to avoid redundant API calls.
// An optional HTTP client can be provided for custom transport (e.g., for metrics tracking).
func DiscoverGitHubAppInstallationID(ctx context.Context, appId int64, privateKey, enterpriseBaseURL, org string, httpClient ...*http.Client) (int64, error) {
	domain, err := domainFromBaseURL(enterpriseBaseURL)
	if err != nil {
		return 0, fmt.Errorf("failed to get domain from base URL: %w", err)
	}
	org = strings.ToLower(org)
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%d", strings.ToLower(org), domain, appId)
	if id, found := githubInstallationIdCache.Get(cacheKey); found {
		return id.(int64), nil
	}

	// Use provided HTTP client or default
	var transport http.RoundTripper
	if len(httpClient) > 0 && httpClient[0] != nil && httpClient[0].Transport != nil {
		transport = httpClient[0].Transport
	} else {
		transport = http.DefaultTransport
	}

	// Create GitHub App transport
	rt, err := ghinstallation.NewAppsTransport(transport, appId, []byte(privateKey))
	if err != nil {
		return 0, fmt.Errorf("failed to create GitHub app transport: %w", err)
	}

	if enterpriseBaseURL != "" {
		rt.BaseURL = enterpriseBaseURL
	}

	// Create GitHub client
	var client *github.Client
	clientTransport := &http.Client{Transport: rt}
	if enterpriseBaseURL == "" {
		client = github.NewClient(clientTransport)
	} else {
		client, err = github.NewClient(clientTransport).WithEnterpriseURLs(enterpriseBaseURL, enterpriseBaseURL)
		if err != nil {
			return 0, fmt.Errorf("failed to create GitHub enterprise client: %w", err)
		}
	}

	// List all installations and cache them
	var allInstallations []*github.Installation
	opts := &github.ListOptions{PerPage: 100}

	// Lock for the entire loop to avoid multiple concurrent API calls on startup
	githubInstallationIdCacheMutex.Lock()
	defer githubInstallationIdCacheMutex.Unlock()

	// Check cache again inside the write lock in case another goroutine already fetched it
	if id, found := githubInstallationIdCache.Get(cacheKey); found {
		return id.(int64), nil
	}

	for {
		installations, resp, err := client.Apps.ListInstallations(ctx, opts)
		if err != nil {
			return 0, fmt.Errorf("failed to list installations: %w", err)
		}

		allInstallations = append(allInstallations, installations...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Cache all installation IDs
	for _, installation := range allInstallations {
		if installation.Account != nil && installation.Account.Login != nil && installation.ID != nil {
			githubInstallationIdCache.Set(cacheKey, *installation.ID, gocache.DefaultExpiration)
		}
	}

	// Return the installation ID for the requested org
	if id, found := githubInstallationIdCache.Get(cacheKey); found {
		return id.(int64), nil
	}
	return 0, fmt.Errorf("installation not found for org: %s", org)
}

// domainFromBaseURL extracts the host (domain) from the given GitHub base URL.
// Supports HTTP(S), SSH URLs, and git@host:org/repo forms.
// Returns an error if a domain cannot be extracted.
func domainFromBaseURL(baseURL string) (string, error) {
	if baseURL == "" {
		return "github.com", nil
	}

	// --- 1. SSH-style Git URL: git@github.com:org/repo.git ---
	if strings.Contains(baseURL, "@") && strings.Contains(baseURL, ":") && !strings.Contains(baseURL, "://") {
		parts := strings.SplitN(baseURL, "@", 2)
		right := parts[len(parts)-1]             // github.com:org/repo
		host := strings.SplitN(right, ":", 2)[0] // github.com
		if host != "" {
			return host, nil
		}
		return "", fmt.Errorf("failed to extract host from SSH-style URL: %q", baseURL)
	}

	// --- 2. Ensure scheme so url.Parse works ---
	if !strings.HasPrefix(baseURL, "http://") &&
		!strings.HasPrefix(baseURL, "https://") &&
		!strings.HasPrefix(baseURL, "ssh://") {
		baseURL = "https://" + baseURL
	}

	// --- 3. Standard URL parse ---
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL %q: %w", baseURL, err)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("URL %q parsed but host is empty", baseURL)
	}

	host := parsed.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host, nil
}

// ExtractOrgFromRepoURL extracts the organization/owner name from a GitHub repository URL.
// Supports formats:
//   - HTTPS: https://github.com/org/repo.git
//   - SSH: git@github.com:org/repo.git
//   - SSH with port: git@github.com:22/org/repo.git or ssh://git@github.com:22/org/repo.git
func ExtractOrgFromRepoURL(repoURL string) (string, error) {
	if repoURL == "" {
		return "", errors.New("repo URL is empty")
	}

	// Handle edge case: ssh://git@host:org/repo (malformed but used in practice)
	// This format mixes ssh:// prefix with colon notation instead of using a slash.
	// Convert it to git@host:org/repo which git-urls can parse correctly.
	// We distinguish this from the valid ssh://git@host:22/org/repo (with port number).
	if strings.HasPrefix(repoURL, "ssh://git@") {
		remainder := strings.TrimPrefix(repoURL, "ssh://")
		if _, after, ok := strings.Cut(remainder, ":"); ok {
			afterColon := after
			slashIdx := strings.Index(afterColon, "/")

			// Check if what follows the colon is a port number
			isPort := false
			if slashIdx > 0 {
				if _, err := strconv.Atoi(afterColon[:slashIdx]); err == nil {
					isPort = true
				}
			}

			// If not a port, it's the malformed format - strip ssh:// prefix
			if !isPort && slashIdx != 0 {
				repoURL = remainder
			}
		}
	}

	// Use git-urls library to parse all Git URL formats
	parsed, err := giturls.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse repository URL %q: %w", repoURL, err)
	}

	// Clean the path: remove leading/trailing slashes and .git suffix
	path := strings.Trim(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	if path == "" {
		return "", fmt.Errorf("repository URL %q does not contain a path", repoURL)
	}

	// Extract the first path component (organization/owner)
	// Path format is typically "org/repo" or "org/repo/subpath"
	if idx := strings.Index(path, "/"); idx > 0 {
		org := path[:idx]
		// Normalize to lowercase for case-insensitive comparison
		return strings.ToLower(org), nil
	}

	// If there's no slash, the entire path might be just the org (unusual but handle it)
	// This would fail validation later, but let's return it
	return "", fmt.Errorf("could not extract organization from repository URL %q: path %q does not contain org/repo format", repoURL, path)
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
func (c GoogleCloudCreds) GetUserInfo(_ context.Context) (string, string, error) {
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

	return utilio.NewCloser(func() error {
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
		return "", fmt.Errorf("failed to get SHA256 hash for Google Cloud credentials: %w", err)
	}

	return token.AccessToken, nil
}

var _ Creds = AzureWorkloadIdentityCreds{}

type AzureWorkloadIdentityCreds struct {
	store         CredsStore
	tokenProvider workloadidentity.TokenProvider
}

func NewAzureWorkloadIdentityCreds(store CredsStore, tokenProvider workloadidentity.TokenProvider) AzureWorkloadIdentityCreds {
	return AzureWorkloadIdentityCreds{
		store:         store,
		tokenProvider: tokenProvider,
	}
}

// GetUserInfo returns the username and email address for the credentials, if they're available.
func (creds AzureWorkloadIdentityCreds) GetUserInfo(_ context.Context) (string, string, error) {
	// Email not implemented for HTTPS creds.
	return workloadidentity.EmptyGuid, "", nil
}

func (creds AzureWorkloadIdentityCreds) Environ() (io.Closer, []string, error) {
	token, err := creds.GetAzureDevOpsAccessToken()
	if err != nil {
		return NopCloser{}, nil, err
	}
	nonce := creds.store.Add("", token)
	env := creds.store.Environ(nonce)
	env = append(env, fmt.Sprintf("%s=Authorization: Bearer %s", bearerAuthHeaderEnv, token))

	return utilio.NewCloser(func() error {
		creds.store.Remove(nonce)
		return nil
	}), env, nil
}

func (creds AzureWorkloadIdentityCreds) getAccessToken(scope string) (string, error) {
	// Compute hash of creds for lookup in cache
	key, err := argoutils.GenerateCacheKey("%s", scope)
	if err != nil {
		return "", fmt.Errorf("failed to get SHA256 hash for Azure credentials: %w", err)
	}

	t, found := azureTokenCache.Get(key)
	if found {
		return t.(*workloadidentity.Token).AccessToken, nil
	}

	token, err := creds.tokenProvider.GetToken(scope)
	if err != nil {
		return "", fmt.Errorf("failed to get Azure access token: %w", err)
	}

	cacheExpiry := workloadidentity.CalculateCacheExpiryBasedOnTokenExpiry(token.ExpiresOn)
	if cacheExpiry > 0 {
		azureTokenCache.Set(key, token, cacheExpiry)
	}
	return token.AccessToken, nil
}

func (creds AzureWorkloadIdentityCreds) GetAzureDevOpsAccessToken() (string, error) {
	accessToken, err := creds.getAccessToken(azureDevopsEntraResourceId) // wellknown resourceid of Azure DevOps
	return accessToken, err
}
