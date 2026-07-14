package git

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cenkalti/backoff/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	skeemaknownhosts "github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"

	"github.com/argoproj/argo-cd/v3/common"
	certutil "github.com/argoproj/argo-cd/v3/util/cert"
	"github.com/argoproj/argo-cd/v3/util/env"
	executil "github.com/argoproj/argo-cd/v3/util/exec"
	"github.com/argoproj/argo-cd/v3/util/proxy"
	"github.com/argoproj/argo-cd/v3/util/versions"
)

var (
	ErrInvalidRepoURL   = errors.New("repo URL is invalid")
	ErrNoNoteFound      = errors.New("no note found")
	ErrRevisionNotFound = errors.New("revision not found")
)

var errOptimizedLsRemoteTimeout = errors.New("optimized git ls-remote timed out")

// builtinGitConfig configuration contains statements that are needed
// for correct ArgoCD operation. These settings will override any
// user-provided configuration of same options.
var builtinGitConfig = map[string]string{
	"maintenance.autoDetach": "false",
	"gc.autoDetach":          "false",
}

// BuiltinGitConfigEnv contains builtin git configuration in the
// format acceptable by Git.
var BuiltinGitConfigEnv []string

// CommitMetadata contains metadata about a commit that is related in some way to another commit.
type CommitMetadata struct {
	// Author is the author of the commit.
	// Comes from the Argocd-reference-commit-author trailer.
	Author mail.Address
	// Date is the date of the commit, formatted as by `git show -s --format=%aI`.
	// May be an empty string if the date is unknown.
	// Comes from the Argocd-reference-commit-date trailer.
	Date string
	// Subject is the commit message subject, i.e. `git show -s --format=%s`.
	// Comes from the Argocd-reference-commit-subject trailer.
	Subject string
	// Body is the commit message body, excluding the subject, i.e. `git show -s --format=%b`.
	// Comes from the Argocd-reference-commit-body trailer.
	Body string
	// SHA is the commit hash.
	// Comes from the Argocd-reference-commit-sha trailer.
	SHA string
	// RepoURL is the URL of the repository where the commit is located.
	// Comes from the Argocd-reference-commit-repourl trailer.
	// This value is not validated beyond confirming that it's a URL, and it should not be used to construct UI links
	// unless it is properly validated and/or sanitized first.
	RepoURL string
}

// RevisionReference contains a reference to a some information that is related in some way to another commit. For now,
// it supports only references to a commit. In the future, it may support other types of references.
type RevisionReference struct {
	// Commit contains metadata about the commit that is related in some way to another commit.
	Commit *CommitMetadata
}

type RevisionMetadata struct {
	// Author is the author of the commit. Corresponds to the output of `git log -n 1 --pretty='format:%an <%ae>'`.
	Author string
	// Date is the date of the commit. Corresponds to the output of `git log -n 1 --pretty='format:%ad'`.
	Date time.Time
	Tags []string
	// Message is the commit message.
	Message string
	// References contains metadata about information that is related in some way to this commit. This data comes from
	// git commit trailers starting with "Argocd-reference-". We currently only support a single reference to a commit,
	// but we return an array to allow for future expansion.
	References []RevisionReference
}

// this should match reposerver/repository/repository.proto/RefsList
type Refs struct {
	Branches []string
	Tags     []string
	// heads and remotes are also refs, but are not needed at this time.
}

type gitRefCache interface {
	SetGitReferences(repo string, references []*plumbing.Reference) error
	GetOrLockGitReferences(repo string, lockId string, references *[]*plumbing.Reference) (string, error)
	UnlockGitReferences(repo string, lockId string) error
}

// Client is a generic git client interface
type Client interface {
	Root() string
	RepoURL() string
	Init() error
	Fetch(ctx context.Context, revision string, depth int64) error
	Submodule(ctx context.Context) error
	Checkout(ctx context.Context, revision string, submoduleEnabled bool, cleanState bool) (string, error)
	LsRefs() (*Refs, error)
	LsRemote(revision string) (string, error)
	LsFiles(ctx context.Context, path string, enableNewGitFileGlobbing bool) ([]string, error)
	LsLargeFiles(ctx context.Context) ([]string, error)
	CommitSHA(ctx context.Context) (string, error)
	RevisionMetadata(ctx context.Context, revision string) (*RevisionMetadata, error)
	// Deprecated: To be removed in the next major version when Signature verification is replaced with Source Integrity.
	VerifyCommitSignature(ctx context.Context, revision string) (string, error)
	// IsAnnotatedTag determines if the revision is, or resolves to an annotated tag.
	IsAnnotatedTag(ctx context.Context, revision string) bool
	// LsSignatures gets a list of revisions including their GPG signature info.
	// If revision is an annotated tag or a semantic constraint matching an annotated tag, its signature is reported as well
	// If deep==true, list the commits backwards in history until a signed "seal commit" or repo init commit. The listing includes those seal commits.
	// If deep==false, examines the revision only. Checking the annotated tag signature if the revision is an annotated tag, commit signature otherwise.
	LsSignatures(ctx context.Context, revision string, deep bool) ([]RevisionSignatureInfo, string, error)
	ChangedFiles(ctx context.Context, revision string, targetRevision string) ([]string, error)
	IsRevisionPresent(ctx context.Context, revision string) bool
	// SetAuthor sets the author name and email in the git configuration.
	SetAuthor(ctx context.Context, name, email string) (string, error)
	// CheckoutOrOrphan checks out the branch. If the branch does not exist, it creates an orphan branch.
	CheckoutOrOrphan(ctx context.Context, branch string, submoduleEnabled bool) (string, error)
	// CheckoutOrNew checks out the given branch. If the branch does not exist, it creates an empty branch based on
	// the base branch.
	CheckoutOrNew(ctx context.Context, branch, base string, submoduleEnabled bool) (string, error)
	// RemoveContents removes all files from the given paths in the git repository.
	RemoveContents(ctx context.Context, paths []string) (string, error)
	// CommitAndPush commits and pushes changes to the target branch.
	CommitAndPush(ctx context.Context, branch, message string) (string, error)
	// GetCommitNote gets the note associated with the DRY sha stored in the specific namespace
	GetCommitNote(ctx context.Context, sha string, namespace string) (string, error)
	// AddAndPushNote adds a note to a DRY sha and then pushes it.
	AddAndPushNote(ctx context.Context, sha string, namespace string, note string) error
	// HasFileChanged returns the outout of git diff considering whether it is tracked or un-tracked
	HasFileChanged(ctx context.Context, filePath string) (bool, error)
}

type EventHandlers struct {
	OnLsRemote func(repo string) func()
	OnFetch    func(repo string) func()
	OnPush     func(repo string) func()
}

// nativeGitClient implements Client interface using git CLI
type nativeGitClient struct {
	EventHandlers

	// URL of the repository
	repoURL string
	// Root path of repository
	root string
	// Authenticator credentials for private repositories
	creds Creds
	// Whether to connect insecurely to repository, e.g. don't verify certificate
	insecure bool
	// Whether the repository is LFS enabled
	enableLfs bool
	// gitRefCache knows how to cache git refs
	gitRefCache gitRefCache
	// indicates if client allowed to load refs from cache
	loadRefFromCache bool
	// HTTP/HTTPS proxy used to access repository
	proxy string
	// list of targets that shouldn't use the proxy, applies only if the proxy is set
	noProxy string
	// git configuration environment variables
	gitConfigEnv []string
	// tagPrefix filters git tags to only those with this prefix when resolving semver constraints.
	// The prefix is stripped before comparison and re-added to the resolved tag name.
	tagPrefix string
	// optimizedLsRemoteEnabled uses native git ls-remote with server-side ref narrowing when possible.
	optimizedLsRemoteEnabled bool
	// optimizedLsRemoteRefPrefixes controls which ref namespaces are eligible for optimized ls-remote.
	optimizedLsRemoteRefPrefixes []string
}

type runOpts struct {
	SkipErrorLogging bool
	CaptureStderr    bool
}

var (
	maxAttemptsCount = 1
	maxRetryDuration time.Duration
	retryDuration    time.Duration
	factor           int64
)

func init() {
	if countStr := os.Getenv(common.EnvGitAttemptsCount); countStr != "" {
		cnt, err := strconv.Atoi(countStr)
		if err != nil {
			panic(fmt.Sprintf("Invalid value in %s env variable: %v", common.EnvGitAttemptsCount, err))
		}
		maxAttemptsCount = int(math.Max(float64(cnt), 1))
	}

	maxRetryDuration = env.ParseDurationFromEnv(common.EnvGitRetryMaxDuration, common.DefaultGitRetryMaxDuration, 0, math.MaxInt64)
	retryDuration = env.ParseDurationFromEnv(common.EnvGitRetryDuration, common.DefaultGitRetryDuration, 0, math.MaxInt64)
	factor = env.ParseInt64FromEnv(common.EnvGitRetryFactor, common.DefaultGitRetryFactor, 0, math.MaxInt64)

	BuiltinGitConfigEnv = append(BuiltinGitConfigEnv, fmt.Sprintf("GIT_CONFIG_COUNT=%d", len(builtinGitConfig)))
	idx := 0
	for k, v := range builtinGitConfig {
		BuiltinGitConfigEnv = append(BuiltinGitConfigEnv, fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", idx, k))
		BuiltinGitConfigEnv = append(BuiltinGitConfigEnv, fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", idx, v))
		idx++
	}
}

type ClientOpts func(c *nativeGitClient)

// WithCache sets git revisions cacher as well as specifies if client should tries to use cached resolved revision
func WithCache(cache gitRefCache, loadRefFromCache bool) ClientOpts {
	return func(c *nativeGitClient) {
		c.gitRefCache = cache
		c.loadRefFromCache = loadRefFromCache
	}
}

func WithBuiltinGitConfig(enable bool) ClientOpts {
	return func(c *nativeGitClient) {
		if enable {
			c.gitConfigEnv = BuiltinGitConfigEnv
		} else {
			c.gitConfigEnv = nil
		}
	}
}

// WithEventHandlers sets the git client event handlers
func WithEventHandlers(handlers EventHandlers) ClientOpts {
	return func(c *nativeGitClient) {
		c.EventHandlers = handlers
	}
}

// WithTagPrefix sets a tag prefix to filter and strip when resolving semver constraints via LsRemote.
// Only tags with this prefix are considered; the prefix is stripped before comparison and re-added to the result.
func WithTagPrefix(prefix string) ClientOpts {
	return func(c *nativeGitClient) {
		c.tagPrefix = prefix
	}
}

// WithOptimizedLsRemote enables native git ls-remote calls for configured ref prefixes.
func WithOptimizedLsRemote(enabled bool, refPrefixes []string) ClientOpts {
	return func(c *nativeGitClient) {
		c.optimizedLsRemoteEnabled = enabled
		c.optimizedLsRemoteRefPrefixes = normalizeOptimizedLsRemoteRefPrefixes(refPrefixes)
	}
}

func NewClient(rawRepoURL string, creds Creds, insecure bool, enableLfs bool, proxy string, noProxy string, opts ...ClientOpts) (Client, error) {
	r := regexp.MustCompile(`([/:])`)
	normalizedGitURL := NormalizeGitURL(rawRepoURL)
	if normalizedGitURL == "" {
		return nil, fmt.Errorf("repository %q cannot be initialized: %w", SanitizeRepoURL(rawRepoURL), ErrInvalidRepoURL)
	}
	root := filepath.Join(os.TempDir(), r.ReplaceAllString(normalizedGitURL, "_"))
	if root == os.TempDir() {
		return nil, fmt.Errorf("repository %q cannot be initialized, because its root would be system temp at %s", SanitizeRepoURL(rawRepoURL), root)
	}
	return NewClientExt(rawRepoURL, root, creds, insecure, enableLfs, proxy, noProxy, opts...)
}

func NewClientExt(rawRepoURL string, root string, creds Creds, insecure bool, enableLfs bool, proxy string, noProxy string, opts ...ClientOpts) (Client, error) {
	client := &nativeGitClient{
		repoURL:      rawRepoURL,
		root:         root,
		creds:        creds,
		insecure:     insecure,
		enableLfs:    enableLfs,
		proxy:        proxy,
		noProxy:      noProxy,
		gitConfigEnv: BuiltinGitConfigEnv,
	}
	for i := range opts {
		opts[i](client)
	}
	return client, nil
}

var gitClientTimeout = env.ParseDurationFromEnv("ARGOCD_GIT_REQUEST_TIMEOUT", 15*time.Second, 0, math.MaxInt64)

// gitCleanupGracePeriod is the minimum age a temporary pack file must reach
// before cleanupOrphanedTempPackfiles will remove it. A fetch is killed at
// ARGOCD_EXEC_TIMEOUT (plus the fatal-timeout grace), so twice that comfortably
// exceeds the longest a fetch can be in flight; anything older cannot belong to
// a live fetch (for example a concurrent fetch from another repo-server replica
// sharing an RWX cache volume).
var gitCleanupGracePeriod = 2 * env.ParseDurationFromEnv("ARGOCD_EXEC_TIMEOUT", 90*time.Second, 0, math.MaxInt64)

// Returns a HTTP client object suitable for go-git to use using the following
// pattern:
//   - If insecure is true, always returns a client with certificate verification
//     turned off.
//   - If one or more custom certificates are stored for the repository, returns
//     a client with those certificates in the list of root CAs used to verify
//     the server's certificate.
//   - Otherwise (and on non-fatal errors), a default HTTP client is returned.
func GetRepoHTTPClient(repoURL string, insecure bool, creds Creds, proxyURL string, noProxy string) *http.Client {
	// Default HTTP client
	customHTTPClient := &http.Client{
		// 15 second timeout by default
		Timeout: gitClientTimeout,
		// don't follow redirect
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	proxyFunc := proxy.GetCallback(proxyURL, noProxy)

	// Callback function to return any configured client certificate
	// We never return err, but an empty cert instead.
	clientCertFunc := func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		var err error
		cert := tls.Certificate{}

		// If we aren't called with GenericHTTPSCreds, then we just return an empty cert
		httpsCreds, ok := creds.(GenericHTTPSCreds)
		if !ok {
			return &cert, nil
		}

		// If the creds contain client certificate data, we return a TLS.Certificate
		// populated with the cert and its key.
		if httpsCreds.HasClientCert() {
			cert, err = tls.X509KeyPair([]byte(httpsCreds.GetClientCertData()), []byte(httpsCreds.GetClientCertKey()))
			if err != nil {
				log.Errorf("Could not load Client Certificate: %v", err)
				return &cert, nil
			}
		}

		return &cert, nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxyFunc
	transport.TLSClientConfig = &tls.Config{
		GetClientCertificate: clientCertFunc,
	}
	transport.DisableKeepAlives = true
	customHTTPClient.Transport = transport
	if insecure {
		transport.TLSClientConfig.InsecureSkipVerify = true
		return customHTTPClient
	}
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return customHTTPClient
	}
	serverCertificatePem, err := certutil.GetCertificateForConnect(parsedURL.Host)
	if err != nil {
		return customHTTPClient
	}
	if len(serverCertificatePem) > 0 {
		certPool := certutil.GetCertPoolFromPEMData(serverCertificatePem)
		transport.TLSClientConfig.RootCAs = certPool
	}
	return customHTTPClient
}

// resolveSSHHostKeyConfig returns a HostKeyCallback bound to Argo CD's
// ssh_known_hosts file together with the HostKeyAlgorithms registered for the
// given repo URL. Populating HostKeyAlgorithms is required to avoid
// "knownhosts: key mismatch" handshake failures with go-git v5.16+ (see
// go-git/go-git#1551).
func resolveSSHHostKeyConfig(repoURL string) (ssh.HostKeyCallback, []string, error) {
	db, err := skeemaknownhosts.NewDB(certutil.GetSSHKnownHostsDataPath())
	if err != nil {
		return nil, nil, err
	}
	var algos []string
	if hostWithPort := SSHHostWithPort(repoURL); hostWithPort != "" {
		algos = db.HostKeyAlgorithms(hostWithPort)
	}
	return db.HostKeyCallback(), algos, nil
}

// buildSSHAuth returns a go-git SSH AuthMethod for repoURL. When creds is non
// nil the supplied private key is used; otherwise the auth falls back to the
// local ssh-agent (mirroring go-git's DefaultAuthBuilder). In both cases
// host-key verification is wired against Argo CD's ssh_known_hosts file rather
// than the user's ~/.ssh/known_hosts, and HostKeyAlgorithms is populated to
// avoid "knownhosts: key mismatch" with go-git v5.16+ (go-git/go-git#1551).
func buildSSHAuth(repoURL string, creds *SSHCreds) (transport.AuthMethod, error) {
	user := ""
	if isSSH, u := IsSSHURL(repoURL); isSSH {
		user = u
	}

	// Insecure mode short-circuits known_hosts verification entirely.
	if creds != nil && creds.insecure {
		signer, err := ssh.ParsePrivateKey([]byte(creds.sshPrivateKey))
		if err != nil {
			return nil, err
		}
		auth := &PublicKeysWithOptions{}
		auth.User = user
		auth.Signer = signer
		auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		return auth, nil
	}

	cb, algos, err := resolveSSHHostKeyConfig(repoURL)
	if err != nil {
		// Returning the error rather than continuing with a nil callback
		// avoids handing back an AuthMethod with no host-key verification.
		// For the no-credentials path, newAuth catches this and lets go-git
		// fall back to its DefaultAuthBuilder.
		return nil, fmt.Errorf("could not set up SSH known hosts callback for %s: %w", SanitizeRepoURL(repoURL), err)
	}

	if creds == nil {
		// No explicit credentials: use ssh-agent, same as go-git's default,
		// but with our known_hosts wired in.
		agentAuth, err := gitssh.NewSSHAgentAuth(user)
		if err != nil {
			return nil, err
		}
		agentAuth.HostKeyCallback = cb
		agentAuth.HostKeyAlgorithms = algos
		return agentAuth, nil
	}

	signer, err := ssh.ParsePrivateKey([]byte(creds.sshPrivateKey))
	if err != nil {
		return nil, err
	}
	auth := &PublicKeysWithOptions{}
	auth.User = user
	auth.Signer = signer
	auth.HostKeyCallback = cb
	// PublicKeysWithOptions.ClientConfig sets cfg.HostKeyAlgorithms from the
	// wrapper field, then go-git's SetHostKeyCallback overwrites it from the
	// embedded helper field — so both must be populated.
	auth.HostKeyAlgorithms = algos
	auth.PublicKeys.HostKeyAlgorithms = algos
	return auth, nil
}

func newAuth(repoURL string, creds Creds) (transport.AuthMethod, error) {
	switch creds := creds.(type) {
	case SSHCreds:
		return buildSSHAuth(repoURL, &creds)
	case HTTPSCreds:
		if creds.bearerToken != "" {
			return &githttp.TokenAuth{Token: creds.bearerToken}, nil
		}
		auth := githttp.BasicAuth{Username: creds.username, Password: creds.password}
		if auth.Username == "" {
			auth.Username = "x-access-token"
		}
		return &auth, nil
	case GitHubAppCreds:
		token, err := creds.getAccessToken()
		if err != nil {
			return nil, err
		}
		auth := githttp.BasicAuth{Username: "x-access-token", Password: token}
		return &auth, nil
	case GoogleCloudCreds:
		username, err := creds.getUsername()
		if err != nil {
			return nil, fmt.Errorf("failed to get username from creds: %w", err)
		}
		token, err := creds.getAccessToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get access token from creds: %w", err)
		}

		auth := githttp.BasicAuth{Username: username, Password: token}
		return &auth, nil
	case AzureWorkloadIdentityCreds:
		token, err := creds.GetAzureDevOpsAccessToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get access token from creds: %w", err)
		}

		auth := githttp.TokenAuth{Token: token}
		return &auth, nil
	case AzureServicePrincipalCreds:
		token, err := creds.getAccessToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get access token from creds: %w", err)
		}
		auth := githttp.TokenAuth{Token: token}
		return &auth, nil
	}

	// Without explicit credentials, go-git's DefaultAuthBuilder would fall
	// back to ssh-agent and read known_hosts from ~/.ssh / $SSH_KNOWN_HOSTS,
	// ignoring Argo CD's ssh_known_hosts ConfigMap. Build the same auth
	// ourselves so we can wire in the Argo CD-managed known_hosts.
	if isSSH, _ := IsSSHURL(repoURL); isSSH {
		auth, err := buildSSHAuth(repoURL, nil)
		if err != nil {
			log.Debugf("falling back to go-git default SSH auth for %s: %v", repoURL, err)
			return nil, nil
		}
		return auth, nil
	}

	return nil, nil
}

func (m *nativeGitClient) Root() string {
	return m.root
}

func (m *nativeGitClient) RepoURL() string {
	return m.repoURL
}

// Init initializes a local git repository and sets the remote origin
func (m *nativeGitClient) Init() error {
	_, err := git.PlainOpen(m.root)
	if err == nil {
		return nil
	}
	if !errors.Is(err, git.ErrRepositoryNotExists) {
		return err
	}
	log.Infof("Initializing %s to %s", m.repoURL, m.root)
	err = os.RemoveAll(m.root)
	if err != nil {
		return fmt.Errorf("unable to clean repo at %s: %w", m.root, err)
	}
	err = os.MkdirAll(m.root, 0o755)
	if err != nil {
		return err
	}
	repo, err := git.PlainInit(m.root, false)
	if err != nil {
		return err
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: git.DefaultRemoteName,
		URLs: []string{m.repoURL},
	})
	return err
}

// IsLFSEnabled returns true if the repository is LFS enabled
func (m *nativeGitClient) IsLFSEnabled() bool {
	return m.enableLfs
}

func (m *nativeGitClient) fetch(ctx context.Context, revision string, depth int64) error {
	args := []string{"fetch", "origin"}
	if revision != "" {
		args = append(args, revision)
	}

	if depth > 0 {
		args = append(args, "--depth", strconv.FormatInt(depth, 10))
	} else {
		args = append(args, "--tags")
	}
	args = append(args, "--force", "--prune")
	return m.runCredentialedCmd(ctx, args...)
}

// IsRevisionPresent checks to see if the given revision already exists locally.
func (m *nativeGitClient) IsRevisionPresent(ctx context.Context, revision string) bool {
	if revision == "" {
		return false
	}

	cmd := exec.CommandContext(ctx, "git", "cat-file", "-t", revision)
	out, err := m.runCmdOutput(cmd, runOpts{SkipErrorLogging: true})
	if out == "commit" && err == nil {
		return true
	}
	return false
}

// cleanupOrphanedTempPackfiles removes leftover objects/pack/tmp_{pack,idx,rev,mtimes}_* files
// produced by a git fetch/index-pack that was killed (for example by the exec
// timeout) before it could finalize the pack. Git treats these as garbage and
// never prunes them itself, so without this cleanup they accumulate on every
// failed fetch into the reused cache directory and can grow the repo-server
// volume without bound. This is best-effort: failures are logged, not returned.
//
// Within a single repo-server the per-repository lock (reposerver/repository/
// lock.go) already serializes fetch/checkout per cache directory, so no
// in-process fetch is writing these files when we get here. To stay safe across
// processes too (for example several repo-server replicas sharing an RWX cache
// volume), only files older than a grace window are removed, so a temp file that
// a concurrent fetch is still writing is never deleted.
func (m *nativeGitClient) cleanupOrphanedTempPackfiles() {
	// git's index-pack streams these temp files into objects/pack/ during a fetch
	// and renames them to pack-<hash>.* on finalize; a killed fetch strands them.
	// This is a best-effort superset across git versions (tmp_rev_/tmp_mtimes_
	// are newer). The receive-pack quarantine dir (tmp_objdir-*) is a directory
	// and is skipped below, so it is intentionally not listed here.
	tempPrefixes := []string{"tmp_pack_", "tmp_idx_", "tmp_rev_", "tmp_mtimes_"}
	packDir := filepath.Join(m.root, ".git", "objects", "pack")
	entries, err := os.ReadDir(packDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warnf("git cleanup: cannot read pack dir %s: %v", packDir, err)
		}
		return
	}

	var removed int
	var reclaimed int64
	for _, entry := range entries {
		// Only remove git's interrupted-fetch temp files (the tempPrefixes
		// above); finalized pack-*.{pack,idx,rev} files and any subdirectories
		// must be left untouched.
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		isTempPack := false
		for _, prefix := range tempPrefixes {
			if strings.HasPrefix(name, prefix) {
				isTempPack = true
				break
			}
		}
		if !isTempPack {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			// Can't determine the age, so don't risk deleting a live temp file.
			continue
		}
		if time.Since(info.ModTime()) < gitCleanupGracePeriod {
			// Still within the grace window: a concurrent fetch may be writing
			// it. Leave it; a later sweep reclaims it once it is stale.
			continue
		}
		path := filepath.Join(packDir, name)
		if rerr := os.Remove(path); rerr != nil {
			log.Warnf("git cleanup: failed to remove orphaned temp pack %s: %v", path, rerr)
			continue
		}
		removed++
		reclaimed += info.Size()
	}

	if removed > 0 {
		log.Infof("git cleanup: removed %d orphaned temp pack file(s) (%d bytes) for %s", removed, reclaimed, m.repoURL)
	}
}

// Fetch fetches latest updates from origin
func (m *nativeGitClient) Fetch(ctx context.Context, revision string, depth int64) error {
	if m.OnFetch != nil {
		done := m.OnFetch(m.repoURL)
		defer done()
	}

	err := m.fetch(ctx, revision, depth)
	if err != nil {
		m.cleanupOrphanedTempPackfiles()
		return err
	}

	// When we have LFS support enabled, check for large files and fetch them too.
	if m.IsLFSEnabled() {
		largeFiles, err := m.LsLargeFiles(ctx)
		if err == nil && len(largeFiles) > 0 {
			err = m.runCredentialedCmd(ctx, "lfs", "fetch", "--all")
			if err != nil {
				return err
			}
		}
	}

	return err
}

// LsFiles lists the local working tree, including only files that are under source control
func (m *nativeGitClient) LsFiles(ctx context.Context, path string, enableNewGitFileGlobbing bool) ([]string, error) {
	if enableNewGitFileGlobbing {
		// This is the new way with safer globbing

		// evaluating the root path for symlinks
		realRoot, err := filepath.EvalSymlinks(m.root)
		if err != nil {
			return nil, err
		}
		// searching for the pattern inside the root path
		allFiles, err := doublestar.FilepathGlob(filepath.Join(realRoot, path))
		if err != nil {
			return nil, err
		}
		var files []string
		for _, file := range allFiles {
			link, err := filepath.EvalSymlinks(file)
			if err != nil {
				return nil, err
			}
			absPath, err := filepath.Abs(link)
			if err != nil {
				return nil, err
			}

			if strings.HasPrefix(absPath, realRoot) {
				// removing the repository root prefix from the file path
				relativeFile, err := filepath.Rel(realRoot, file)
				if err != nil {
					return nil, err
				}
				files = append(files, relativeFile)
			} else {
				log.Warnf("Absolute path for %s is outside of repository, ignoring it", file)
			}
		}
		return files, nil
	}
	// This is the old and default way
	out, err := m.runCmd(ctx, "ls-files", "--full-name", "-z", "--", path)
	if err != nil {
		return nil, err
	}
	// remove last element, which is blank regardless of whether we're using nullbyte or newline
	ss := strings.Split(out, "\000")
	return ss[:len(ss)-1], nil
}

// LsLargeFiles lists all files that have references to LFS storage
func (m *nativeGitClient) LsLargeFiles(ctx context.Context) ([]string, error) {
	out, err := m.runCmd(ctx, "lfs", "ls-files", "-n")
	if err != nil {
		return nil, err
	}
	ss := strings.Split(out, "\n")
	return ss, nil
}

// Submodule embed other repositories into this repository
func (m *nativeGitClient) Submodule(ctx context.Context) error {
	if err := m.runCredentialedCmd(ctx, "submodule", "sync", "--recursive"); err != nil {
		return err
	}
	return m.runCredentialedCmd(ctx, "submodule", "update", "--init", "--recursive")
}

// Checkout checks out the specified revision
func (m *nativeGitClient) Checkout(ctx context.Context, revision string, submoduleEnabled bool, cleanState bool) (string, error) {
	if revision == "" || revision == "HEAD" {
		revision = "origin/HEAD"
	}
	if out, err := m.runCmd(ctx, "checkout", "--force", revision); err != nil {
		return out, fmt.Errorf("failed to checkout %s: %w", revision, err)
	}
	// We must populate LFS content by using lfs checkout, if we have at least
	// one LFS reference in the current revision.
	if m.IsLFSEnabled() {
		largeFiles, err := m.LsLargeFiles(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list LFS files: %w", err)
		}
		if len(largeFiles) > 0 {
			if out, err := m.runCmd(ctx, "lfs", "checkout"); err != nil {
				return out, fmt.Errorf("failed to checkout LFS files: %w", err)
			}
		}
	}
	if _, err := os.Stat(m.root + "/.gitmodules"); !os.IsNotExist(err) {
		if submoduleEnabled {
			if err := m.Submodule(ctx); err != nil {
				return "", fmt.Errorf("failed to update submodules: %w", err)
			}
		}
	}
	if cleanState || submoduleEnabled {
		// NOTE
		// The double “f” in the arguments is not a typo: the first “f” tells
		// `git clean` to delete untracked files and directories, and the second “f”
		// tells it to clean untracked nested Git repositories (for example a
		// submodule which has since been removed).
		if out, err := m.runCmd(ctx, "clean", "-ffdx"); err != nil {
			return out, fmt.Errorf("failed to clean: %w", err)
		}
	}
	return "", nil
}

func (m *nativeGitClient) getRefsFromCacheOrFetch(cacheKey string, logPrefix string, fetch func() ([]*plumbing.Reference, error)) ([]*plumbing.Reference, error) {
	myLockUUID, err := uuid.NewRandom()
	myLockID := ""
	if err != nil {
		log.Debugf("Error generating %s git references cache lock id: %v", logPrefix, err)
	} else {
		myLockID = myLockUUID.String()
	}

	needsUnlock := false
	if m.gitRefCache != nil && m.loadRefFromCache {
		var res []*plumbing.Reference
		foundLockID, err := m.gitRefCache.GetOrLockGitReferences(cacheKey, myLockID, &res)
		isLockOwner := myLockID == foundLockID
		if !isLockOwner && err == nil {
			// Valid value already in cache
			return res, nil
		} else if !isLockOwner && err != nil {
			// Error getting value from cache
			log.Debugf("Error getting %s git references from cache: %v", logPrefix, err)
			return nil, err
		}
		needsUnlock = true
		// Defer a soft reset of the cache lock, if the value is set this call will be ignored
		defer func() {
			if needsUnlock {
				err := m.gitRefCache.UnlockGitReferences(cacheKey, myLockID)
				if err != nil {
					log.Debugf("Error unlocking %s git references from cache: %v", logPrefix, err)
				}
			}
		}()
	}

	res, err := fetch()
	if err == nil && m.gitRefCache != nil {
		if err := m.gitRefCache.SetGitReferences(cacheKey, res); err != nil {
			log.Warnf("Failed to store %s git references to cache: %v", logPrefix, err)
		} else {
			// Since we successfully overwrote the lock with valid data, we don't need to unlock
			needsUnlock = false
		}
	}
	return res, err
}

func (m *nativeGitClient) getRefs() ([]*plumbing.Reference, error) {
	return m.getRefsFromCacheOrFetch(m.repoURL, "full", func() ([]*plumbing.Reference, error) {
		if m.OnLsRemote != nil {
			done := m.OnLsRemote(m.repoURL)
			defer done()
		}

		repo, err := git.Init(memory.NewStorage(), nil)
		if err != nil {
			return nil, err
		}
		remote, err := repo.CreateRemote(&config.RemoteConfig{
			Name: git.DefaultRemoteName,
			URLs: []string{m.repoURL},
		})
		if err != nil {
			return nil, err
		}
		auth, err := newAuth(m.repoURL, m.creds)
		if err != nil {
			return nil, err
		}
		return listRemote(remote, &git.ListOptions{Auth: auth}, m.insecure, m.creds, m.proxy, m.noProxy)
	})
}

func (m *nativeGitClient) LsRefs() (*Refs, error) {
	refs, err := m.getRefs()
	if err != nil {
		return nil, err
	}

	sortedRefs := &Refs{
		Branches: []string{},
		Tags:     []string{},
	}

	for _, revision := range refs {
		if revision.Name().IsBranch() {
			sortedRefs.Branches = append(sortedRefs.Branches, revision.Name().Short())
		} else if revision.Name().IsTag() {
			sortedRefs.Tags = append(sortedRefs.Tags, revision.Name().Short())
		}
	}

	log.Debugf("LsRefs resolved %d branches and %d tags on repository", len(sortedRefs.Branches), len(sortedRefs.Tags))

	// Would prefer to sort by last modified date but that info does not appear to be available without resolving each ref
	sort.Strings(sortedRefs.Branches)
	sort.Strings(sortedRefs.Tags)

	return sortedRefs, nil
}

// LsRemote resolves the commit SHA of a specific branch, tag (with semantic versioning or not),
// or HEAD. If the supplied revision does not resolve, and "looks" like a 7+ hexadecimal commit SHA,
// it will return the revision string. Otherwise, it returns an error indicating that the revision could
// not be resolved. This method runs with in-memory storage and is safe to run concurrently,
// or to be run without a git repository locally cloned.
func (m *nativeGitClient) LsRemote(revision string) (res string, err error) {
	for attempt := 0; attempt < maxAttemptsCount; attempt++ {
		res, err = m.lsRemote(revision)
		if err == nil {
			return res, nil
		} else if apierrors.IsInternalError(err) || apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) ||
			apierrors.IsTooManyRequests(err) || utilnet.IsProbableEOF(err) || utilnet.IsConnectionReset(err) {
			// Formula: timeToWait = duration * factor^retry_number
			// Note that timeToWait should equal to duration for the first retry attempt.
			// When timeToWait is more than maxDuration retry should be performed at maxDuration.
			timeToWait := float64(retryDuration) * (math.Pow(float64(factor), float64(attempt)))
			if maxRetryDuration > 0 {
				timeToWait = math.Min(float64(maxRetryDuration), timeToWait)
			}
			time.Sleep(time.Duration(timeToWait))
		}
	}
	return res, err
}

func getGitTags(refs []*plumbing.Reference) []string {
	var tags []string
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tags = append(tags, ref.Name().Short())
		}
	}
	return tags
}

func normalizeOptimizedLsRemoteRefPrefixes(refPrefixes []string) []string {
	normalized := make([]string, 0, len(refPrefixes))
	seen := map[string]bool{}
	for _, prefix := range refPrefixes {
		prefix = strings.TrimSpace(prefix)
		switch prefix {
		case "refs/heads":
			prefix = "refs/heads/"
		case "refs/tags":
			prefix = "refs/tags/"
		}
		if prefix == "" || seen[prefix] {
			continue
		}
		seen[prefix] = true
		normalized = append(normalized, prefix)
	}
	return normalized
}

func (m *nativeGitClient) optimizedLsRemoteCapabilities() (heads bool, tags bool) {
	for _, prefix := range m.optimizedLsRemoteRefPrefixes {
		switch prefix {
		case "refs/heads/":
			heads = true
		case "refs/tags/":
			tags = true
		}
	}
	return heads, tags
}

func (m *nativeGitClient) optimizedLsRemoteRefPrefixPlan() ([]string, []string, bool) {
	heads, tags := m.optimizedLsRemoteCapabilities()
	if !heads && !tags {
		return nil, nil, false
	}

	args := []string{"ls-remote"}
	cacheParts := []string{"HEAD"}
	if heads {
		args = append(args, "--heads")
		cacheParts = append(cacheParts, "heads")
	}
	if tags {
		args = append(args, "--tags")
		cacheParts = append(cacheParts, "tags")
	}
	args = append(args, m.repoURL)
	return args, cacheParts, true
}

func (m *nativeGitClient) optimizedLsRemoteCacheKey(parts ...string) string {
	return fmt.Sprintf("ls-remote-optimized|%s|%s", m.repoURL, strings.Join(parts, ","))
}

func (m *nativeGitClient) getOptimizedLsRemoteRefs(cacheKey string, fetch func() ([]*plumbing.Reference, error)) ([]*plumbing.Reference, error) {
	return m.getRefsFromCacheOrFetch(cacheKey, "optimized", fetch)
}

func (m *nativeGitClient) lsRemoteOptimized(revision string) (string, bool, error) {
	if !m.optimizedLsRemoteEnabled {
		return "", false, nil
	}

	if revision == "" {
		revision = "HEAD"
	}

	args, cacheParts, ok := m.optimizedLsRemoteRefPrefixPlan()
	if !ok {
		return "", false, nil
	}
	heads, tags := m.optimizedLsRemoteCapabilities()

	if strings.HasPrefix(revision, "refs/") {
		switch {
		case strings.HasPrefix(revision, "refs/heads/"):
			if !heads {
				return "", false, nil
			}
		case strings.HasPrefix(revision, "refs/tags/"):
			if !tags {
				return "", false, nil
			}
		default:
			return "", false, nil
		}
	}

	cacheKey := m.optimizedLsRemoteCacheKey(cacheParts...)
	fetchedBulkRefs := false
	fetchBulkRefs := func() ([]*plumbing.Reference, error) {
		fetchedBulkRefs = true
		return m.runLsRemote(args...)
	}
	refs, err := m.getOptimizedLsRemoteRefs(cacheKey, fetchBulkRefs)
	if err != nil {
		if errors.Is(err, errOptimizedLsRemoteTimeout) {
			return "", true, err
		}
		return "", false, err
	}
	res, err := m.resolveRevisionWithoutTruncatedSHAFallback(revision, refs)
	if err != nil && revision == "HEAD" {
		// --heads and --tags provide protocol v2 server-side narrowing, but exclude HEAD.
		// Fetch HEAD only when requested; if the bulk listing came from cache, refresh it
		// first so HEAD and its target branch cannot resolve to different commits.
		if !fetchedBulkRefs {
			refs, err = fetchBulkRefs()
			if err != nil {
				if errors.Is(err, errOptimizedLsRemoteTimeout) {
					return "", true, err
				}
				return "", false, err
			}
		}
		headRefs, headErr := m.runLsRemote("ls-remote", m.repoURL, "HEAD")
		if headErr != nil {
			if errors.Is(headErr, errOptimizedLsRemoteTimeout) {
				return "", true, headErr
			}
			return "", false, headErr
		}
		refs = append(refs, headRefs...)
		if m.gitRefCache != nil {
			if cacheErr := m.gitRefCache.SetGitReferences(cacheKey, refs); cacheErr != nil {
				log.Warnf("Failed to add HEAD to optimized git references cache: %v", cacheErr)
			}
		}
		res, err = m.resolveRevisionWithoutTruncatedSHAFallback(revision, refs)
	}
	if err != nil {
		if m.optimizedLsRemoteResolveMissNeedsFallback(revision, heads, tags) {
			return "", false, nil
		}
		return "", true, err
	}
	return res, true, nil
}

func (m *nativeGitClient) optimizedLsRemoteResolveMissNeedsFallback(revision string, heads bool, tags bool) bool {
	if IsTruncatedCommitSHA(revision) {
		return true
	}
	if revision == "HEAD" || revision == "" {
		return false
	}
	if strings.HasPrefix(revision, "refs/") {
		switch {
		case strings.HasPrefix(revision, "refs/heads/"):
			return !heads
		case strings.HasPrefix(revision, "refs/tags/"):
			return !tags
		default:
			return true
		}
	}
	return true
}

func (m *nativeGitClient) runLsRemote(args ...string) ([]*plumbing.Reference, error) {
	if m.OnLsRemote != nil {
		done := m.OnLsRemote(m.repoURL)
		defer done()
	}

	ctx, cancel := context.WithTimeout(context.Background(), gitClientTimeout)
	defer cancel()

	out, err := m.runCredentialedCmdOutput(ctx, append([]string{"-c", "protocol.version=2"}, args...)...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w after %s: %w", errOptimizedLsRemoteTimeout, gitClientTimeout, err)
		}
		return nil, err
	}
	return parseLsRemoteOutput(out)
}

func parseLsRemoteOutput(out string) ([]*plumbing.Reference, error) {
	refsByName := map[plumbing.ReferenceName]*plumbing.Reference{}
	var orderedNames []plumbing.ReferenceName

	addRef := func(ref *plumbing.Reference) {
		name := ref.Name()
		if _, ok := refsByName[name]; !ok {
			orderedNames = append(orderedNames, name)
		}
		refsByName[name] = ref
	}

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if after, ok := strings.CutPrefix(line, "ref: "); ok {
			targetName, refName, ok := strings.Cut(after, "\t")
			if !ok {
				return nil, fmt.Errorf("malformed ls-remote symbolic ref line: %q", line)
			}
			addRef(plumbing.NewSymbolicReference(plumbing.ReferenceName(refName), plumbing.ReferenceName(targetName)))
			continue
		}

		hash, refName, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("malformed ls-remote ref line: %q", line)
		}
		refName = strings.TrimSuffix(refName, "^{}")
		addRef(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.NewHash(hash)))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	refs := make([]*plumbing.Reference, 0, len(orderedNames))
	for _, name := range orderedNames {
		refs = append(refs, refsByName[name])
	}
	return refs, nil
}

func (m *nativeGitClient) lsRemote(revision string) (string, error) {
	if IsCommitSHA(revision) {
		return revision, nil
	}

	if res, ok, err := m.lsRemoteOptimized(revision); ok {
		return res, err
	} else if err != nil {
		log.Debugf("optimized ls-remote failed for revision '%s', falling back to default resolver: %v", revision, err)
	}

	refs, err := m.getRefs()
	if err != nil {
		return "", fmt.Errorf("failed to list refs: %w", err)
	}

	return m.resolveRevision(revision, refs)
}

func (m *nativeGitClient) resolveRevision(revision string, refs []*plumbing.Reference) (string, error) {
	return m.resolveRevisionWithOptions(revision, refs, true)
}

func (m *nativeGitClient) resolveRevisionWithoutTruncatedSHAFallback(revision string, refs []*plumbing.Reference) (string, error) {
	return m.resolveRevisionWithOptions(revision, refs, false)
}

func (m *nativeGitClient) resolveRevisionWithOptions(revision string, refs []*plumbing.Reference, allowTruncatedSHAFallback bool) (string, error) {
	if revision == "" {
		revision = "HEAD"
	}

	maxV, err := versions.MaxVersion(revision, getGitTags(refs), m.tagPrefix)
	if err == nil {
		revision = maxV
	}

	// refToHash keeps a maps of remote refs to their hash
	// (e.g. refs/heads/master -> a67038ae2e9cb9b9b16423702f98b41e36601001)
	refToHash := make(map[string]string)

	// refToResolve remembers ref name of the supplied revision if we determine the revision is a
	// symbolic reference (like HEAD), in which case we will resolve it from the refToHash map
	refToResolve := ""

	isShortRef := IsShortRef(revision)
	log.Debugf("Attempting to resolve revision '%s' (is short ref: %t)", revision, isShortRef)

	for _, ref := range refs {
		refName := ref.Name().String()
		hash := ref.Hash().String()
		if ref.Type() == plumbing.HashReference {
			refToHash[refName] = hash
		}
		// log.Debugf("%s\t%s", hash, refName)
		if (isShortRef && ref.Name().Short() == revision) || refName == revision {
			if ref.Type() == plumbing.HashReference {
				log.Debugf("revision '%s' resolved to '%s'", revision, hash)
				return hash, nil
			}
			if ref.Type() == plumbing.SymbolicReference {
				refToResolve = ref.Target().String()
			}
		}
	}

	if refToResolve != "" {
		// If refToResolve is non-empty, we are resolving symbolic reference (e.g. HEAD).
		// It should exist in our refToHash map
		if hash, ok := refToHash[refToResolve]; ok {
			log.Debugf("symbolic reference '%s' (%s) resolved to '%s'", revision, refToResolve, hash)
			return hash, nil
		}
	}

	// We support the ability to use a truncated commit-SHA (e.g. first 7 characters of a SHA)
	if allowTruncatedSHAFallback && IsTruncatedCommitSHA(revision) {
		log.Debugf("revision '%s' assumed to be commit sha", revision)
		return revision, nil
	}

	// If we get here, revision string had non hexadecimal characters (indicating its a branch, tag,
	// or symbolic ref) and we were unable to resolve it to a commit SHA.
	return "", fmt.Errorf("unable to resolve '%s' to a commit SHA: %w", revision, ErrRevisionNotFound)
}

// CommitSHA returns current commit sha from `git rev-parse HEAD`
func (m *nativeGitClient) CommitSHA(ctx context.Context) (string, error) {
	out, err := m.runCmd(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// RevisionMetadata returns the meta-data for the commit
func (m *nativeGitClient) RevisionMetadata(ctx context.Context, revision string) (*RevisionMetadata, error) {
	out, err := m.runCmd(ctx, "show", "-s", "--format=%an <%ae>%n%at%n%B", revision)
	if err != nil {
		return nil, err
	}
	segments := strings.SplitN(out, "\n", 3)
	if len(segments) != 3 {
		return nil, fmt.Errorf("expected 3 segments, got %v", segments)
	}
	author := segments[0]
	authorDateUnixTimestamp, _ := strconv.ParseInt(segments[1], 10, 64)
	message := strings.TrimSpace(segments[2])

	cmd := exec.CommandContext(ctx, "git", "interpret-trailers", "--parse")
	cmd.Stdin = strings.NewReader(message)
	out, err = m.runCmdOutput(cmd, runOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to interpret trailers for revision %q in repo %q: %w", revision, SanitizeRepoURL(m.repoURL), err)
	}
	relatedCommits, _ := GetReferences(log.WithFields(log.Fields{"repo": m.repoURL, "revision": revision}), out)

	out, err = m.runCmd(ctx, "tag", "--points-at", revision)
	if err != nil {
		return nil, err
	}
	tags := strings.Fields(out)

	return &RevisionMetadata{
		Author:     author,
		Date:       time.Unix(authorDateUnixTimestamp, 0),
		Tags:       tags,
		Message:    message,
		References: relatedCommits,
	}, nil
}

func truncate(str string) string {
	if utf8.RuneCountInString(str) > 100 {
		return string([]rune(str)[0:97]) + "..."
	}
	return str
}

var shaRegex = regexp.MustCompile(`^[0-9a-f]{5,40}$`)

// GetReferences extracts related commit metadata from the commit message trailers. If referenced commit
// metadata is present, we return a slice containing a single metadata object. If no related commit metadata is found,
// we return a nil slice.
//
// If a trailer fails validation, we log an error and skip that trailer. We truncate the trailer values to 100
// characters to avoid excessively long log messages.
//
// We also return the commit message body with all valid Argocd-reference-commit-* trailers removed.
func GetReferences(logCtx *log.Entry, commitMessageBody string) ([]RevisionReference, string) {
	unrelatedLines := strings.Builder{}
	var relatedCommit CommitMetadata
	scanner := bufio.NewScanner(strings.NewReader(commitMessageBody))
	for scanner.Scan() {
		line := scanner.Text()
		updated := updateCommitMetadata(logCtx, &relatedCommit, line)
		if !updated {
			unrelatedLines.WriteString(line + "\n")
		}
	}
	var relatedCommits []RevisionReference
	if relatedCommit != (CommitMetadata{}) {
		relatedCommits = append(relatedCommits, RevisionReference{
			Commit: &relatedCommit,
		})
	}
	return relatedCommits, unrelatedLines.String()
}

// updateCommitMetadata checks if the line is a valid Argocd-reference-commit-* trailer. If so, it updates
// the relatedCommit object and returns true. If the line is not a valid trailer, it returns false.
func updateCommitMetadata(logCtx *log.Entry, relatedCommit *CommitMetadata, line string) bool {
	if !strings.HasPrefix(line, "Argocd-reference-commit-") {
		return false
	}
	parts := strings.SplitN(line, ": ", 2)
	if len(parts) != 2 {
		return false
	}
	trailerKey := parts[0]
	trailerValue := parts[1]
	switch trailerKey {
	case "Argocd-reference-commit-repourl":
		_, err := url.Parse(trailerValue)
		if err != nil {
			logCtx.Errorf("failed to parse repo URL %q: %v", truncate(trailerValue), err)
			return false
		}
		relatedCommit.RepoURL = trailerValue
	case "Argocd-reference-commit-author":
		address, err := mail.ParseAddress(trailerValue)
		if err != nil || address == nil {
			logCtx.Errorf("failed to parse author email %q: %v", truncate(trailerValue), err)
			return false
		}
		relatedCommit.Author = *address
	case "Argocd-reference-commit-date":
		// Validate that it's the correct date format.
		t, err := time.Parse(time.RFC3339, trailerValue)
		if err != nil {
			logCtx.Errorf("failed to parse date %q with RFC3339 format: %v", truncate(trailerValue), err)
			return false
		}
		relatedCommit.Date = t.Format(time.RFC3339)
	case "Argocd-reference-commit-subject":
		relatedCommit.Subject = trailerValue
	case "Argocd-reference-commit-body":
		body := ""
		err := json.Unmarshal([]byte(trailerValue), &body)
		if err != nil {
			logCtx.Errorf("failed to parse body %q as JSON: %v", truncate(trailerValue), err)
			return false
		}
		relatedCommit.Body = body
	case "Argocd-reference-commit-sha":
		if !shaRegex.MatchString(trailerValue) {
			logCtx.Errorf("invalid commit SHA %q in trailer %s: must be a lowercase hex string 5-40 characters long", truncate(trailerValue), trailerKey)
			return false
		}
		relatedCommit.SHA = trailerValue
	default:
		return false
	}
	return true
}

// VerifyCommitSignature Runs verify-commit on a given revision and returns the output
//
// Deprecated: To be removed in the next major version when Signature verification is replaced with Source Integrity.
func (m *nativeGitClient) VerifyCommitSignature(ctx context.Context, revision string) (string, error) {
	cmd := m.cmdWithGPG(ctx, "git-verify-wrapper.sh", revision)
	out, err := m.runCmdOutput(cmd, runOpts{})
	if err != nil {
		log.Errorf("error verifying commit signature: %v", err)
		return "", errors.New("permission denied")
	}
	return out, nil
}

type (
	GPGVerificationResult string
	RevisionSignatureInfo struct {
		Revision           string
		VerificationResult GPGVerificationResult
		SignatureKeyID     string
		Date               string
		AuthorIdentity     string
	}
)

const (
	GPGVerificationResultGood             GPGVerificationResult = "signed"                         // All good
	GPGVerificationResultBad              GPGVerificationResult = "bad signature"                  // Not able to cryptographically verify signature
	GPGVerificationResultUntrusted        GPGVerificationResult = "signed with untrusted key"      // The trust level of the key in the gpg keyring is not sufficient
	GPGVerificationResultExpiredSignature GPGVerificationResult = "expired signature"              // Signature have expired
	GPGVerificationResultExpiredKey       GPGVerificationResult = "signed with expired key"        // Signed with a key expired at the time of the signing
	GPGVerificationResultRevokedKey       GPGVerificationResult = "signed with revoked key"        // Signed with a key that is revoked
	GPGVerificationResultMissingKey       GPGVerificationResult = "signed with key not in keyring" // The key used to sign was not added to the gpg keyring
	GPGVerificationResultUnsigned         GPGVerificationResult = "unsigned"                       // Commit it not signed at all
)

func gpgVerificationFromGpgCode(gpgCode string) (GPGVerificationResult, error) {
	// GPG code presented by `git verify-tag --raw`
	// https://github.com/gpg/gnupg/blob/master/doc/DETAILS#general-status-codes
	switch gpgCode {
	case "GOODSIG":
		return GPGVerificationResultGood, nil
	case "BADSIG":
		return GPGVerificationResultBad, nil
	case "EXPSIG":
		return GPGVerificationResultExpiredSignature, nil
	case "EXPKEYSIG":
		return GPGVerificationResultExpiredKey, nil
	case "REVKEYSIG":
		return GPGVerificationResultRevokedKey, nil
	case "ERRSIG":
		return GPGVerificationResultMissingKey, nil
	default:
		return "", fmt.Errorf("unable to parse VerificationResult from '%s'", gpgCode)
	}
}

func gpgVerificationFromGitRevParse(oneLetter string) (GPGVerificationResult, error) {
	// The letters each represent a given verification result, as output by git rev-parse pretty format.
	// See PRETTY FORMAT in git-rev-list(1) for more information.
	// https://github.com/git/git/blob/5e6e4854e086ba0025bc7dc11e6b475c92a2f556/gpg-interface.c#L188
	switch oneLetter {
	case "G":
		return GPGVerificationResultGood, nil
	case "B":
		return GPGVerificationResultBad, nil
	case "U":
		return GPGVerificationResultUntrusted, nil
	case "X":
		return GPGVerificationResultExpiredSignature, nil
	case "Y":
		return GPGVerificationResultExpiredKey, nil
	case "R":
		return GPGVerificationResultRevokedKey, nil
	case "E":
		return GPGVerificationResultMissingKey, nil
	case "N":
		return GPGVerificationResultUnsigned, nil
	default:
		return "", fmt.Errorf("unable to parse VerificationResult from '%s'", oneLetter)
	}
}

var gpgKeyIdRegexp = regexp.MustCompile("[0-9a-zA-Z]{16}")

func (m *nativeGitClient) tagSignature(ctx context.Context, tagRevision string) (*RevisionSignatureInfo, error) {
	// Unlike for commits, there is no elegant way to slurp all signature info for tag. So this extracts details needed
	// for RevisionSignatureInfo from 2 different git invocations.
	cmd := m.cmdWithGPG(ctx, "git", "for-each-ref", "refs/tags/"+tagRevision, `--format=%(taggerdate),%(taggername) "%(taggeremail)"`)
	tagOut, err := m.runCmdOutput(cmd, runOpts{})
	if err != nil {
		return nil, err
	}
	if tagOut == "" {
		return nil, fmt.Errorf("no tag found: %q", tagRevision)
	}
	tagInfo := strings.Split(tagOut, ",")
	if len(tagInfo) != 2 {
		return nil, fmt.Errorf("failed to parse tag %q for revisions %q", tagOut, tagRevision)
	}

	cmd = m.cmdWithGPG(ctx, "git", "verify-tag", tagRevision, "--raw")
	tagGpgOut, err := m.runCmdOutput(cmd, runOpts{
		CaptureStderr:    true, // The structured --raw output is printed to stderr only
		SkipErrorLogging: true, // Unsigned returns rc=1
	})
	status, keyId, err := evaluateGpgSignStatus(err, tagGpgOut)
	if err != nil {
		return nil, fmt.Errorf("gpg failed verifying git tag %q: %s", tagRevision, err.Error())
	}
	info, err := newRevisionSignatureInfo(tagRevision, status, keyId, tagInfo[0], tagInfo[1])
	if err != nil {
		return nil, fmt.Errorf("failed building revision gpg signature info for tag %q: %s", tagRevision, err.Error())
	}
	return info, err
}

func evaluateGpgSignStatus(cmdErr error, tagGpgOut string) (result GPGVerificationResult, keyId string, err error) {
	if cmdErr != nil {
		// Commit is not signed
		if tagGpgOut == "error: no signature found" {
			return GPGVerificationResultUnsigned, "", nil
		}

		// Parse the output to extract info, ERRSIG causes `rc!=0`
		if !strings.Contains(tagGpgOut, "[GNUPG:] ERRSIG ") {
			return "", "", cmdErr
		}
	}

	// https://github.com/gpg/gnupg/blob/master/doc/DETAILS#general-status-codes
	re := regexp.MustCompile(`\[GNUPG:] (GOODSIG|BADSIG|EXPSIG|EXPKEYSIG|REVKEYSIG|ERRSIG) ([0-9A-F]+) `)
	for line := range strings.Lines(tagGpgOut) {
		match := re.FindAllStringSubmatch(line, -1)
		switch len(match) {
		case 0:
			continue
		case 1:
			result, err := gpgVerificationFromGpgCode(match[0][1])
			if err != nil {
				return "", "", err
			}
			return result, match[0][2], nil
		default:
			return "", "", fmt.Errorf("too many matches parsing line %q", line)
		}
	}

	return "", "", fmt.Errorf("unexpected `git verify-tag --raw` output: %q", tagGpgOut)
}

func (m *nativeGitClient) LsSignatures(ctx context.Context, unresolvedRevision string, deep bool) ([]RevisionSignatureInfo, string, error) {
	legacyVerification := ""

	// Resolve eventual semantic tag constraint before annotated tag detection
	if versions.IsConstraint(unresolvedRevision) {
		refs, err := m.getRefs()
		if err != nil {
			return nil, "", err
		}
		unresolvedRevision, err = versions.MaxVersion(unresolvedRevision, getGitTags(refs), m.tagPrefix)
		if err != nil {
			return nil, "", err
		}
	}

	legacyVerification, err := m.VerifyCommitSignature(ctx, unresolvedRevision)
	if err != nil {
		return nil, "", err
	}

	var signatures []RevisionSignatureInfo
	if m.IsAnnotatedTag(ctx, unresolvedRevision) {
		signature, err := m.tagSignature(ctx, unresolvedRevision)
		if err != nil {
			return nil, "", err
		}
		signatures = append(signatures, *signature)

		// Check just the annotated tag
		if !deep {
			return signatures, legacyVerification, nil
		}
	}

	commitSignaturesRawOut, err := m.listRawSignatures(ctx, deep)
	if err != nil {
		return nil, "", err
	}

	// Final LF will be cut by executil
	csvR := csv.NewReader(strings.NewReader(commitSignaturesRawOut))
	for {
		r, err := csvR.Read()
		// EOF means parsing had ended
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		if len(r) < 5 {
			return nil, "", fmt.Errorf("invalid rev-list output for %q (fields=%d)", unresolvedRevision, len(r))
		}

		revision := r[0]
		result, err := gpgVerificationFromGitRevParse(r[1])
		if err != nil {
			return nil, "", err
		}
		signatureInfo, err := newRevisionSignatureInfo(revision, result, r[2], r[3], r[4])
		if err != nil {
			return nil, "", fmt.Errorf("failed building revision gpg signature info for %q at %q: %s", unresolvedRevision, revision, err.Error())
		}
		signatures = append(signatures, *signatureInfo)
	}

	return signatures, legacyVerification, nil
}

// newRevisionSignatureInfo builds valid RevisionSignatureInfo
func newRevisionSignatureInfo(revision string, verificationResult GPGVerificationResult, signatureKeyID string, date string, authorIdentity string) (*RevisionSignatureInfo, error) {
	if revision == "" {
		return nil, errors.New("no revision specified")
	}
	if date == "" {
		return nil, errors.New("no date specified")
	}
	if authorIdentity == "" {
		return nil, errors.New("no author specified")
	}
	// Unsigned have no key ID, other states must have key ID
	if verificationResult == GPGVerificationResultUnsigned {
		if signatureKeyID != "" {
			return nil, fmt.Errorf("a gpg signing key id %q specified for unsigned commit", signatureKeyID)
		}
	} else {
		if !gpgKeyIdRegexp.MatchString(signatureKeyID) {
			return nil, fmt.Errorf("invalid gpg signing key %q", signatureKeyID)
		}
	}

	return &RevisionSignatureInfo{
		Revision:           revision,
		VerificationResult: verificationResult,
		SignatureKeyID:     signatureKeyID,
		Date:               date,
		AuthorIdentity:     authorIdentity,
	}, nil
}

func (m *nativeGitClient) listRawSignatures(ctx context.Context, deep bool) (string, error) {
	revisionSha, err := m.CommitSHA(ctx)
	if err != nil {
		return "", err
	}

	// This is using a two-step approach to solve the following problem: find all ancestors of a given revision in git history DAG,
	// stopping on a signed seal commit, or an init commit. Note there might be multiple seal commits that separate the revision
	// form init commit in case the history merged past the most recent seal commit.
	//
	// 1) Find all seal commits based on the trailer in their message. This searches the entire git history, which is unnecessary,
	//    but there does not seem to be a decent way to stop on the most recent seal commits in each branch with a single git invocation.
	//    Found commits are later eliminated to the correctly signed and trusted ones - this is to make sure that unsigned
	//    or untrusted commits with a seal trailer do not stop the history verification.
	// 2) Find all the ancestor commits from the given revision stopping on any of the identified seal commits.

	// See git-rev-list(1) for description of the format string

	var commitFilterArgs []string
	if deep {
		// Find all seal commits with their signing indicator
		cmd := m.cmdWithGPG(ctx, "git", "rev-list", `--pretty=format:%G?,%H`, "--no-commit-header", "--grep=Argocd-gpg-seal:", "--regexp-ignore-case", revisionSha)
		sealCommitsRawOut, err := m.runCmdOutput(cmd, runOpts{})
		if err != nil {
			return "", err
		}

		commitFilterArgs, err = m.getSealRevListFilter(ctx, revisionSha, sealCommitsRawOut)
		if err != nil {
			return "", err
		}
	} else {
		// List only the one revision - no seal commit search done
		commitFilterArgs = []string{revisionSha, "-1", "--"}
	}

	// Find all commits until the criteria, including
	lsArgs := append([]string{"rev-list", `--pretty=format:%H,%G?,%GK,"%aD","%an <%ae>"`, "--no-commit-header"}, commitFilterArgs...)
	commitSignaturesRawOut, err := m.runCmdOutput(m.cmdWithGPG(ctx, "git", lsArgs...), runOpts{})
	if err != nil {
		return "", err
	}
	return commitSignaturesRawOut, nil
}

// getSealRevListFilter create arguments for `git rev-list` to search the history all the way until the seal commits found.
func (m *nativeGitClient) getSealRevListFilter(ctx context.Context, revision string, sealCommitsRawOut string) ([]string, error) {
	// Keep only seal commits with a valid signature
	var sealCommits []string
	for line := range strings.SplitSeq(sealCommitsRawOut, "\n") {
		if strings.HasPrefix(line, "G,") {
			sealCommits = append(sealCommits, line[2:])
		}
	}
	sealCommitsLen := len(sealCommits)
	log.Debugf("Found %d seal commits for %s", sealCommitsLen, revision)

	// No (correctly signed) seal commits found - verify all ancestry
	if sealCommitsLen == 0 {
		return []string{revision, "--"}, nil
	}

	// Resolve, in case revision is not a commit number
	sha, err := m.CommitSHA(ctx)
	if err != nil {
		return nil, err
	}
	if sha == sealCommits[0] {
		// Currently on seal commit - verify just this one
		return []string{revision, "-1", "--"}, nil
	}

	// Some seal commits in history - filter until those
	return append([]string{"--boundary", revision, "--not"}, sealCommits...), nil
}

// IsAnnotatedTag returns true if the revision is an annotated tag existing in the repository, and false for everything else.
func (m *nativeGitClient) IsAnnotatedTag(ctx context.Context, revision string) bool {
	cmd := exec.CommandContext(ctx, "git", "cat-file", "-t", revision)
	out, err := m.runCmdOutput(cmd, runOpts{SkipErrorLogging: true})
	// a lightweight tag returns "commit" - makes sense in the git world
	if err == nil && out == "tag" {
		return true
	}
	return false
}

// ChangedFiles returns a list of files changed between two revisions
func (m *nativeGitClient) ChangedFiles(ctx context.Context, revision string, targetRevision string) ([]string, error) {
	if revision == targetRevision {
		return []string{}, nil
	}

	if !IsCommitSHA(revision) || !IsCommitSHA(targetRevision) {
		return []string{}, errors.New("invalid revision provided, must be SHA")
	}

	out, err := m.runCmd(ctx, "diff", "--name-only", fmt.Sprintf("%s..%s", revision, targetRevision))
	if err != nil {
		return nil, fmt.Errorf("failed to diff %s..%s: %w", revision, targetRevision, err)
	}

	if out == "" {
		return []string{}, nil
	}

	files := strings.Split(out, "\n")
	return files, nil
}

// config runs a git config command.
func (m *nativeGitClient) config(ctx context.Context, args ...string) (string, error) {
	args = append([]string{"config"}, args...)
	out, err := m.runCmd(ctx, args...)
	if err != nil {
		return out, fmt.Errorf("failed to run git config: %w", err)
	}
	return out, nil
}

// SetAuthor sets the author name and email in the git configuration.
func (m *nativeGitClient) SetAuthor(ctx context.Context, name, email string) (string, error) {
	if name != "" {
		out, err := m.config(ctx, "--local", "user.name", name)
		if err != nil {
			return out, err
		}
	}
	if email != "" {
		out, err := m.config(ctx, "--local", "user.email", email)
		if err != nil {
			return out, err
		}
	}
	return "", nil
}

// CheckoutOrOrphan checks out the branch. If the branch does not exist, it creates an orphan branch.
func (m *nativeGitClient) CheckoutOrOrphan(ctx context.Context, branch string, submoduleEnabled bool) (string, error) {
	out, err := m.Checkout(ctx, branch, submoduleEnabled, true)
	if err != nil {
		// If the branch doesn't exist, create it as an orphan branch.
		if !strings.Contains(err.Error(), "did not match any file(s) known to git") {
			return out, fmt.Errorf("failed to checkout branch: %w", err)
		}
		out, err = m.runCmd(ctx, "switch", "--orphan", branch)
		if err != nil {
			return out, fmt.Errorf("failed to create orphan branch: %w", err)
		}

		// Make an empty initial commit.
		out, err = m.runCmd(ctx, "commit", "--allow-empty", "-m", "Initial commit for "+branch)
		if err != nil {
			return out, fmt.Errorf("failed to commit initial commit: %w", err)
		}

		// Push the commit.
		err = m.runCredentialedCmd(ctx, "push", "origin", branch)
		if err != nil {
			return "", fmt.Errorf("failed to push to branch: %w", err)
		}
	}
	return "", nil
}

// CheckoutOrNew checks out the given branch. If the branch does not exist, it creates an empty branch based on
// the base branch.
func (m *nativeGitClient) CheckoutOrNew(ctx context.Context, branch, base string, submoduleEnabled bool) (string, error) {
	out, err := m.Checkout(ctx, branch, submoduleEnabled, true)
	if err != nil {
		if !strings.Contains(err.Error(), "did not match any file(s) known to git") {
			return out, fmt.Errorf("failed to checkout branch: %w", err)
		}
		// If the branch does not exist, create any empty branch based on the sync branch
		// First, checkout the sync branch.
		out, err = m.Checkout(ctx, base, submoduleEnabled, true)
		if err != nil {
			return out, fmt.Errorf("failed to checkout sync branch: %w", err)
		}

		out, err = m.runCmd(ctx, "checkout", "-b", branch)
		if err != nil {
			return out, fmt.Errorf("failed to create branch: %w", err)
		}
	}
	return "", nil
}

// RemoveContents removes all files from the path of git repository.
func (m *nativeGitClient) RemoveContents(ctx context.Context, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	args := append([]string{"rm", "-r", "--ignore-unmatch", "--"}, paths...)
	out, err := m.runCmd(ctx, args...)
	if err != nil {
		return out, fmt.Errorf("failed to clear paths %v: %w", paths, err)
	}
	return "", nil
}

// CommitAndPush commits and pushes changes to the target branch.
func (m *nativeGitClient) CommitAndPush(ctx context.Context, branch, message string) (string, error) {
	out, err := m.runCmd(ctx, "add", ".")
	if err != nil {
		return out, fmt.Errorf("failed to add files: %w", err)
	}

	out, err = m.runCmd(ctx, "commit", "-m", message)
	if err != nil {
		if strings.Contains(out, "nothing to commit, working tree clean") {
			return out, nil
		}
		return out, fmt.Errorf("failed to commit: %w", err)
	}

	if m.OnPush != nil {
		done := m.OnPush(m.repoURL)
		defer done()
	}

	err = m.runCredentialedCmd(ctx, "push", "origin", branch)
	if err != nil {
		return "", fmt.Errorf("failed to push: %w", err)
	}

	return "", nil
}

// GetCommitNote gets the note associated with the DRY sha stored in the specific namespace
func (m *nativeGitClient) GetCommitNote(ctx context.Context, sha string, namespace string) (string, error) {
	if strings.TrimSpace(namespace) == "" {
		namespace = "commit"
	}
	// fetch first
	// cli command: git fetch origin refs/notes/source-hydrator:refs/notes/source-hydrator
	notesRef := "refs/notes/" + namespace
	_ = m.runCredentialedCmd(ctx, "fetch", "origin", fmt.Sprintf("%s:%s", notesRef, notesRef)) // Ignore fetch error for best effort

	ref := "--ref=" + namespace
	out, err := m.runCmd(ctx, "notes", ref, "show", sha)
	if err != nil {
		if strings.Contains(err.Error(), "no note found") {
			return out, fmt.Errorf("failed to get commit note: %w", ErrNoNoteFound)
		}
		return out, fmt.Errorf("failed to get commit note: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// AddAndPushNote adds a note to a DRY sha and then pushes it.
// It uses a retry mechanism to handle concurrent note updates from multiple clients.
func (m *nativeGitClient) AddAndPushNote(ctx context.Context, sha string, namespace string, note string) error {
	if namespace == "" {
		namespace = "commit"
	}
	ref := "--ref=" + namespace
	notesRef := "refs/notes/" + namespace

	// Configure exponential backoff with jitter to handle concurrent note updates
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 50 * time.Millisecond
	b.MaxInterval = 1 * time.Second

	attempt := 0
	operation := func() (struct{}, error) {
		attempt++

		// Fetch the latest notes BEFORE adding to merge concurrent updates
		// Use + prefix to force update local ref (safe because we want latest remote notes)
		fetchErr := m.runCredentialedCmd(ctx, "fetch", "origin", fmt.Sprintf("+%s:%s", notesRef, notesRef))
		// Ignore "couldn't find remote ref" errors (notes don't exist yet - first time)
		if fetchErr != nil && !strings.Contains(fetchErr.Error(), "couldn't find remote ref") {
			log.Debugf("Failed to fetch notes (will continue): %v", fetchErr)
		}

		// Add note locally (use -f to overwrite if this specific commit already has a note locally)
		_, err := m.runCmd(ctx, "notes", ref, "add", "-f", "-m", note, sha)
		if err != nil {
			return struct{}{}, backoff.Permanent(fmt.Errorf("failed to add note: %w", err))
		}

		if m.OnPush != nil {
			done := m.OnPush(m.repoURL)
			defer done()
		}

		// Push WITHOUT -f flag to avoid overwriting other notes
		err = m.runCredentialedCmd(ctx, "push", "origin", notesRef)
		if err == nil {
			if attempt > 1 {
				log.Debugf("AddAndPushNote succeeded after %d retries for commit %s", attempt-1, sha)
			}
			return struct{}{}, nil
		}

		log.Debugf("AddAndPushNote push failed (attempt %d): %v", attempt, err)

		// Check if this is a retryable error
		if !isRetryableNotePushError(err.Error()) {
			return struct{}{}, backoff.Permanent(fmt.Errorf("failed to push note: %w", err))
		}

		return struct{}{}, err
	}

	_, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(b),
		backoff.WithMaxElapsedTime(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to push note after retries: %w", err)
	}
	return nil
}

// isRetryableNotePushError reports whether a failed git notes push is caused by a
// concurrent update to the same notes ref and is therefore safe to retry after
// re-fetching the remote ref. Multiple application controller shards can hydrate
// and push to the same refs/notes ref at once, and git reports the resulting
// collision through several different messages depending on the git version and
// server, including "cannot lock ref" when the server already holds the ref lock.
func isRetryableNotePushError(errStr string) bool {
	return strings.Contains(errStr, "fetch first") || // Remote updated after our fetch (concurrent push completed between our fetch and push)
		strings.Contains(errStr, "reference already exists") || // Concurrent push is holding the lock (git server-side lock)
		strings.Contains(errStr, "incorrect old value") || // Git detected our local ref is stale (concurrent update)
		strings.Contains(errStr, "failed to update ref") || // Generic ref update failure that may include transient issues
		strings.Contains(errStr, "cannot lock ref") // Server could not lock the notes ref because a concurrent push from another shard holds it
}

// HasFileChanged returns the outout of git diff considering whether it is tracked or un-tracked
func (m *nativeGitClient) HasFileChanged(ctx context.Context, filePath string) (bool, error) {
	// Step 1: Is it UNTRACKED? (file is new to git)
	_, err := m.runCmd(ctx, "ls-files", "--error-unmatch", filePath)
	if err != nil {
		// File is NOT tracked by git → means it's new/unadded
		return true, nil
	}
	// use git diff --quiet and check exit code .. --cached is to consider files staged for deletion
	_, err = m.runCmd(ctx, "diff", "--quiet", "--", filePath)
	if err == nil {
		return false, nil // No changes
	}
	// Exit code 1 indicates: changes found
	if strings.Contains(err.Error(), "exit status 1") {
		return true, nil
	}
	// always return the actual wrapped error
	return false, fmt.Errorf("git diff failed: %w", err)
}

// cmdWithGPG creates git Cmd with a GPG-enabled environment
func (m *nativeGitClient) cmdWithGPG(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Env, "GNUPGHOME="+common.GetGnuPGHomePath(), "LANG=C")
	return cmd
}

// runCmd is a convenience function to run a command in a given directory and return its output
func (m *nativeGitClient) runCmd(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	return m.runCmdOutput(cmd, runOpts{})
}

func credentialedGitArgs(args []string, environ []string) []string {
	for _, e := range environ {
		if strings.HasPrefix(e, forceBasicAuthHeaderEnv+"=") {
			args = append([]string{"--config-env", "http.extraHeader=" + forceBasicAuthHeaderEnv}, args...)
		} else if strings.HasPrefix(e, bearerAuthHeaderEnv+"=") {
			args = append([]string{"--config-env", "http.extraHeader=" + bearerAuthHeaderEnv}, args...)
		}
	}
	return args
}

// runCredentialedCmd is a convenience function to run a git command with username/password credentials
func (m *nativeGitClient) runCredentialedCmd(ctx context.Context, args ...string) error {
	closer, environ, err := m.creds.Environ()
	if err != nil {
		return err
	}
	defer func() { _ = closer.Close() }()

	args = credentialedGitArgs(args, environ)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(cmd.Env, environ...)
	_, err = m.runCmdOutput(cmd, runOpts{})
	return humanizeAuthPromptError(m.repoURL, err)
}

// gitTerminalPromptDisabledMsg is the substring Git prints when it needs
// credentials it wasn't given and interactive prompts are disabled
// (GIT_TERMINAL_PROMPT=0). It signals a failed git authentication, not an actual
// terminal problem.
const gitTerminalPromptDisabledMsg = "terminal prompts disabled"

// humanizeAuthPromptError rewrites Git's misleading "terminal prompts disabled"
// failure into an authentication error, since the raw message reads as a tty
// problem when the real cause is that no credentials matched the repository URL.
// Any other error is returned unchanged.
func humanizeAuthPromptError(repoURL string, err error) error {
	if err == nil || !strings.Contains(err.Error(), gitTerminalPromptDisabledMsg) {
		return err
	}
	return fmt.Errorf("failed to authenticate to git repository %q: no credentials matched this URL: %w", SanitizeRepoURL(repoURL), err)
}

// runCredentialedCmdOutput is a convenience function to run a git command with credentials and return its output.
func (m *nativeGitClient) runCredentialedCmdOutput(ctx context.Context, args ...string) (string, error) {
	closer, environ, err := m.creds.Environ()
	if err != nil {
		return "", err
	}
	defer func() { _ = closer.Close() }()

	args = credentialedGitArgs(args, environ)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(cmd.Env, environ...)
	// ls-remote does not need the checkout, which may be protected with mode 000 between operations.
	cmd.Dir = os.TempDir()
	return m.runCmdOutput(cmd, runOpts{})
}

func (m *nativeGitClient) runCmdOutput(cmd *exec.Cmd, ropts runOpts) (string, error) {
	if cmd.Dir == "" {
		cmd.Dir = m.root
	}
	cmd.Env = append(os.Environ(), cmd.Env...)
	// Set $HOME to nowhere, so we can execute Git regardless of any external
	// authentication keys (e.g. in ~/.ssh) -- this is especially important for
	// running tests on local machines and/or CircleCI.
	cmd.Env = append(cmd.Env, "HOME=/dev/null")
	// Skip LFS for most Git operations except when explicitly requested
	cmd.Env = append(cmd.Env, "GIT_LFS_SKIP_SMUDGE=1")
	// Disable Git terminal prompts in case we're running with a tty
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=false")
	// Add Git configuration options that are essential for ArgoCD operation
	cmd.Env = append(cmd.Env, m.gitConfigEnv...)

	// For HTTPS repositories, we need to consider insecure repositories as well
	// as custom CA bundles from the cert database.
	if IsHTTPSURL(m.repoURL) {
		if m.insecure {
			cmd.Env = append(cmd.Env, "GIT_SSL_NO_VERIFY=true")
		} else {
			parsedURL, err := url.Parse(m.repoURL)
			// We don't fail if we cannot parse the URL, but log a warning in that
			// case. And we execute the command in a verbatim way.
			if err != nil {
				log.Warnf("runCmdOutput: Could not parse repo URL '%s'", m.repoURL)
			} else {
				caPath, err := certutil.GetCertBundlePathForRepository(parsedURL.Host)
				if err == nil && caPath != "" {
					cmd.Env = append(cmd.Env, "GIT_SSL_CAINFO="+caPath)
				}
			}
		}
	}
	cmd.Env = proxy.UpsertEnv(cmd, m.proxy, m.noProxy)
	opts := executil.ExecRunOpts{
		TimeoutBehavior: executil.TimeoutBehavior{
			Signal:     syscall.SIGTERM,
			ShouldWait: true,
		},
		SkipErrorLogging: ropts.SkipErrorLogging,
		CaptureStderr:    ropts.CaptureStderr,
	}
	return executil.RunWithExecRunOpts(cmd, opts)
}
