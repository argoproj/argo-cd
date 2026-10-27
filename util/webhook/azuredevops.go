package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	certutil "github.com/argoproj/argo-cd/v3/util/cert"
	"github.com/argoproj/argo-cd/v3/util/git"
)

const (
	// adoDiffsAPIVersion pins the Azure DevOps REST API version used for the Diffs endpoint, so the
	// response contract does not change with the server-side default. Moving to a newer version
	// requires a code change.
	// See: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/diffs/get?view=azure-devops-rest-7.1
	adoDiffsAPIVersion = "7.1"

	// adoDiffsMaxPageSize is the maximum number of file changes requested per call to the Diffs API.
	// The endpoint defaults to 100; this value is aligned with the documented maximum of the related
	// Pull Request Iteration Changes endpoint. Pushes exceeding this limit fall back to a full refresh.
	// See: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-request-iteration-changes/get?view=azure-devops-rest-7.1
	adoDiffsMaxPageSize = 2000

	// adoNullSHA is the all-zeros object ID used by Azure DevOps to signal a non-existent commit.
	// It appears as newObjectId on branch deletion events and as oldObjectId on branch creation events.
	// Diffing against this SHA is not meaningful and the Diffs API returns 404 for such requests.
	adoNullSHA = "0000000000000000000000000000000000000000"

	// adoAPITimeout is the timeout for outbound Azure DevOps REST API calls made during webhook processing.
	adoAPITimeout = 10 * time.Second

	// adoRespDrainLimit bounds how much of an unread response body is discarded so that the
	// connection can be returned to the pool of a cached client.
	adoRespDrainLimit = 512 * 1024

	// adoHTTPClientCacheLimit bounds the number of cached HTTP clients. Clients are keyed by the
	// transport settings of a repository, which usually resolve to a single entry per Azure DevOps
	// host, so the limit is only reached after many TLS or proxy configuration changes without a
	// restart. Beyond it, clients are created per request without keep-alives.
	adoHTTPClientCacheLimit = 64
)

var errNoADOCredentials = errors.New("no supported Azure DevOps REST API credentials configured")

type adoDiffsResponse struct {
	Changes            []adoDiffChange `json:"changes"`
	AllChangesIncluded bool            `json:"allChangesIncluded"`
}

type adoDiffChange struct {
	Item       adoDiffItem `json:"item"`
	ChangeType string      `json:"changeType"`
}

type adoDiffItem struct {
	Path     string `json:"path"`
	IsFolder bool   `json:"isFolder"`
}

func (a *ArgoCDWebhookHandler) azureDevOpsWebhookAuthConfigured() bool {
	// The Azure DevOps webhook parser requires basic auth when either value is configured.
	return a.settings.GetWebhookAzureDevOpsUsername() != "" || a.settings.GetWebhookAzureDevOpsPassword() != ""
}

// lookupAndFetchADOChangedFiles returns the files changed by an Azure DevOps push event, or nil when
// they cannot be determined, in which case all matching applications are refreshed.
// The repository lookup acts as the SSRF guard: the Diffs API is only called for repositories, or
// credentials templates, configured in Argo CD.
func (a *ArgoCDWebhookHandler) lookupAndFetchADOChangedFiles(ctx context.Context, repoURL, repoID, shaBefore, shaAfter string) []string {
	if shaBefore == "" || shaAfter == "" || shaBefore == adoNullSHA || shaAfter == adoNullSHA {
		log.Debugf("skipping Azure DevOps changed files lookup for URL %s because the commit range is empty or includes the null SHA used for branch creation or deletion", repoURL)
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, adoAPITimeout)
	defer cancel()
	argoRepo, err := a.lookupRepositoryWithCredsTemplate(ctx, repoURL)
	if err != nil {
		log.Warnf("error finding repository for Azure DevOps webhook URL %s: %v", repoURL, err)
		return nil
	}
	if argoRepo == nil {
		log.Debugf("no repository or credentials template configured for Azure DevOps webhook URL %s, skipping changed files lookup", repoURL)
		return nil
	}
	changedFiles, err := fetchChangedFilesFromADO(ctx, a.adoHTTPClientFactory(argoRepo), argoRepo, repoID, shaBefore, shaAfter)
	if err != nil {
		if errors.Is(err, errNoADOCredentials) {
			log.Warnf("no supported Azure DevOps REST API credentials configured for repository %s; falling back to full refresh", repoURL)
			return nil
		}
		log.Warnf("error fetching changed files from Azure DevOps diffs API: %v", err)
		return nil
	}
	return changedFiles
}

// parseADOBaseURL extracts the base URL used to construct Azure DevOps REST API requests.
// It supports both the modern format (https://dev.azure.com/{org}/{project}/_git/{repo})
// and the legacy format (https://{org}.visualstudio.com/{project}/_git/{repo}).
// SSH URLs (e.g. git@ssh.dev.azure.com:v3/{org}/{project}/{repo}) are not supported
// because SSH keys cannot be used to authenticate the Azure DevOps REST API.
func parseADOBaseURL(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse Azure DevOps URL %q: %w", repoURL, err)
	}
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 3)
	// Legacy format: https://{org}.visualstudio.com/{project}/_git/{repo}
	// The org is in the host; the first path segment is the project.
	if strings.HasSuffix(u.Host, ".visualstudio.com") {
		if len(parts) < 1 || parts[0] == "" {
			return "", fmt.Errorf("cannot extract project from Azure DevOps URL %q", repoURL)
		}
		return fmt.Sprintf("%s://%s/%s", u.Scheme, u.Host, parts[0]), nil
	}
	// Modern format: https://dev.azure.com/{org}/{project}/_git/{repo}
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("cannot extract organization and project from Azure DevOps URL %q", repoURL)
	}
	return fmt.Sprintf("%s://%s/%s/%s", u.Scheme, u.Host, parts[0], parts[1]), nil
}

// setADOAuthHeader sets the Authorization header on the request based on the repository credentials,
// with the same precedence used for Git operations over HTTPS: a bearer token takes precedence over
// basic auth, and basic auth only needs a password, since Azure DevOps ignores the username when
// authenticating with a personal access token.
// Returns true if credentials were applied, false if none are configured.
func setADOAuthHeader(req *http.Request, repo *v1alpha1.Repository) bool {
	if repo.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+repo.BearerToken)
		return true
	}
	if repo.Password != "" {
		req.SetBasicAuth(repo.Username, repo.Password)
		return true
	}
	return false
}

// adoHTTPClientCache reuses HTTP clients, and therefore their connections, across Azure DevOps
// REST API calls made for repositories that share the same transport settings.
type adoHTTPClientCache struct {
	mu      sync.Mutex
	clients map[string]*http.Client
}

func newADOHTTPClientCache() *adoHTTPClientCache {
	return &adoHTTPClientCache{clients: map[string]*http.Client{}}
}

// get returns an HTTP client honoring the TLS and proxy settings of the repository. Credentials are
// not part of the client and are set on each request, so repositories with different credentials
// can share a client.
func (c *adoHTTPClientCache) get(repo *v1alpha1.Repository) *http.Client {
	key := adoHTTPClientCacheKey(repo)
	c.mu.Lock()
	defer c.mu.Unlock()
	if client, ok := c.clients[key]; ok {
		return client
	}
	client := newADOHTTPClient(repo)
	if len(c.clients) >= adoHTTPClientCacheLimit {
		log.Debugf("Azure DevOps HTTP client cache is full, creating an uncached client for %s", repo.Repo)
		return client
	}
	// Repository HTTP clients disable keep-alives because they are created per request, which would
	// otherwise leave idle connections behind. A cached client is reused, so keep-alives let it
	// reuse connections, and idle ones are closed by the transport's idle timeout.
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.DisableKeepAlives = false
	}
	c.clients[key] = client
	return client
}

// adoHTTPClientCacheKey identifies the settings newADOHTTPClient builds a client from, including
// the custom server certificates, so that TLS and proxy configuration changes take effect without
// a restart.
func adoHTTPClientCacheKey(repo *v1alpha1.Repository) string {
	var host string
	var serverCerts []string
	if u, err := url.Parse(repo.Repo); err == nil {
		host = u.Host
		if !repo.IsInsecure() {
			// Errors are ignored, as newADOHTTPClient falls back to the system roots on error.
			serverCerts, _ = certutil.GetCertificateForConnect(host)
		}
	}
	h := sha256.New()
	parts := append([]string{host, strconv.FormatBool(repo.IsInsecure()), repo.Proxy, repo.NoProxy, repo.TLSClientCertData, repo.TLSClientCertKey}, serverCerts...)
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func newADOHTTPClient(repo *v1alpha1.Repository) *http.Client {
	return git.GetRepoHTTPClient(repo.Repo, repo.IsInsecure(), repo.GetGitCreds(&git.NoopCredsStore{}), repo.Proxy, repo.NoProxy)
}

// fetchChangedFilesFromADO retrieves the list of files changed between two commits by calling
// the Azure DevOps Diffs REST API.
// See: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/diffs/get?view=azure-devops-rest-7.1
func fetchChangedFilesFromADO(ctx context.Context, httpClient *http.Client, repo *v1alpha1.Repository, repoID, shaBefore, shaAfter string) ([]string, error) {
	baseURL, err := parseADOBaseURL(repo.Repo)
	if err != nil {
		return nil, err
	}
	params := url.Values{
		"baseVersion":       {shaBefore},
		"baseVersionType":   {"commit"},
		"targetVersion":     {shaAfter},
		"targetVersionType": {"commit"},
		"$top":              {strconv.Itoa(adoDiffsMaxPageSize)},
		"api-version":       {adoDiffsAPIVersion},
	}
	apiURL := fmt.Sprintf("%s/_apis/git/repositories/%s/diffs/commits?%s",
		baseURL, url.PathEscape(repoID), params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("error creating Azure DevOps diffs API request: %w", err)
	}
	if !setADOAuthHeader(req, repo) {
		return nil, errNoADOCredentials
	}
	log.Debugf("fetching changed files from Azure DevOps diffs API: %s", apiURL)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling Azure DevOps diffs API: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, adoRespDrainLimit))
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("azure DevOps diffs API returned unexpected status %d", resp.StatusCode)
	}
	var diffsResp adoDiffsResponse
	if err := json.NewDecoder(resp.Body).Decode(&diffsResp); err != nil {
		return nil, fmt.Errorf("error decoding Azure DevOps diffs API response: %w", err)
	}
	if !diffsResp.AllChangesIncluded {
		return nil, fmt.Errorf("azure DevOps diffs API response was truncated (more than %d files changed); falling back to full refresh", adoDiffsMaxPageSize)
	}
	changedFiles := make([]string, 0, len(diffsResp.Changes))
	for _, c := range diffsResp.Changes {
		if !c.Item.IsFolder && c.Item.Path != "" {
			changedFiles = append(changedFiles, strings.TrimPrefix(c.Item.Path, "/"))
		}
	}
	log.Debugf("Azure DevOps diffs API returned %d changed files between %s..%s", len(changedFiles), shaBefore, shaAfter)
	return changedFiles, nil
}
