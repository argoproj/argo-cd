package git

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"

	"github.com/argoproj/argo-cd/v3/common"
	certutil "github.com/argoproj/argo-cd/v3/util/cert"
	"github.com/argoproj/argo-cd/v3/util/env"
	executil "github.com/argoproj/argo-cd/v3/util/exec"
	"github.com/argoproj/argo-cd/v3/util/proxy"
	"github.com/argoproj/argo-cd/v3/util/versions"
)

var ErrInvalidRepoURL = errors.New("repo URL is invalid")

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
	Init() error
	Fetch(revision string) error
	Submodule() error
	Checkout(revision string, submoduleEnabled bool) (string, error)
	LsRefs() (*Refs, error)
	LsRemote(revision string) (string, error)
	LsFiles(path string, enableNewGitFileGlobbing bool) ([]string, error)
	LsLargeFiles() ([]string, error)
	CommitSHA() (string, error)
	RevisionMetadata(revision string) (*RevisionMetadata, error)
	VerifyCommitSignature(string) (string, error)
	IsAnnotatedTag(string) bool
	ChangedFiles(revision string, targetRevision string) ([]string, error)
	IsRevisionPresent(revision string) bool
	// SetAuthor sets the author name and email in the git configuration.
	SetAuthor(name, email string) (string, error)
	// CheckoutOrOrphan checks out the branch. If the branch does not exist, it creates an orphan branch.
	CheckoutOrOrphan(branch string, submoduleEnabled bool) (string, error)
	// CheckoutOrNew checks out the given branch. If the branch does not exist, it creates an empty branch based on
	// the base branch.
	CheckoutOrNew(branch, base string, submoduleEnabled bool) (string, error)
	// RemoveContents removes all files from the git repository.
	RemoveContents() (string, error)
	// CommitAndPush commits and pushes changes to the target branch.
	CommitAndPush(branch, message string) (string, error)
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
}

type ClientOpts func(c *nativeGitClient)

// WithCache sets git revisions cacher as well as specifies if client should tries to use cached resolved revision
func WithCache(cache gitRefCache, loadRefFromCache bool) ClientOpts {
	return func(c *nativeGitClient) {
		c.gitRefCache = cache
		c.loadRefFromCache = loadRefFromCache
	}
}

// WithEventHandlers sets the git client event handlers
func WithEventHandlers(handlers EventHandlers) ClientOpts {
	return func(c *nativeGitClient) {
		c.EventHandlers = handlers
	}
}

func NewClient(rawRepoURL string, creds Creds, insecure bool, enableLfs bool, proxy string, noProxy string, opts ...ClientOpts) (Client, error) {
	r := regexp.MustCompile(`([/:])`)
	normalizedGitURL := NormalizeGitURL(rawRepoURL)
	if normalizedGitURL == "" {
		return nil, fmt.Errorf("repository %q cannot be initialized: %w", rawRepoURL, ErrInvalidRepoURL)
	}
	root := filepath.Join(os.TempDir(), r.ReplaceAllString(normalizedGitURL, "_"))
	if root == os.TempDir() {
		return nil, fmt.Errorf("repository %q cannot be initialized, because its root would be system temp at %s", rawRepoURL, root)
	}
	return NewClientExt(rawRepoURL, root, creds, insecure, enableLfs, proxy, noProxy, opts...)
}

func NewClientExt(rawRepoURL string, root string, creds Creds, insecure bool, enableLfs bool, proxy string, noProxy string, opts ...ClientOpts) (Client, error) {
	client := &nativeGitClient{
		repoURL:   rawRepoURL,
		root:      root,
		creds:     creds,
		insecure:  insecure,
		enableLfs: enableLfs,
		proxy:     proxy,
		noProxy:   noProxy,
	}
	for i := range opts {
		opts[i](client)
	}
	return client, nil
}

var gitClientTimeout = env.ParseDurationFromEnv("ARGOCD_GIT_REQUEST_TIMEOUT", 15*time.Second, 0, math.MaxInt64)

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
	transport := &http.Transport{
		Proxy: proxyFunc,
		TLSClientConfig: &tls.Config{
			GetClientCertificate: clientCertFunc,
		},
		DisableKeepAlives: true,
	}
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

func newAuth(repoURL string, creds Creds) (transport.AuthMethod, error) {
	switch creds := creds.(type) {
	case SSHCreds:
		var sshUser string
		if isSSH, user := IsSSHURL(repoURL); isSSH {
			sshUser = user
		}
		signer, err := ssh.ParsePrivateKey([]byte(creds.sshPrivateKey))
		if err != nil {
			return nil, err
		}
		auth := &PublicKeysWithOptions{}
		auth.User = sshUser
		auth.Signer = signer
		if creds.insecure {
			auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		} else {
			// Set up validation of SSH known hosts for using our ssh_known_hosts
			// file.
			auth.HostKeyCallback, err = knownhosts.New(certutil.GetSSHKnownHostsDataPath())
			if err != nil {
				log.Errorf("Could not set-up SSH known hosts callback: %v", err)
			}
		}
		return auth, nil
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
	}

	return nil, nil
}

func (m *nativeGitClient) Root() string {
	return m.root
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

func (m *nativeGitClient) fetch(revision string) error {
	var err error
	if revision != "" {
		err = m.runCredentialedCmd("fetch", "origin", revision, "--tags", "--force", "--prune")
	} else {
		err = m.runCredentialedCmd("fetch", "origin", "--tags", "--force", "--prune")
	}
	return err
}

// IsRevisionPresent checks to see if the given revision already exists locally.
func (m *nativeGitClient) IsRevisionPresent(revision string) bool {
	if revision == "" {
		return false
	}

	cmd := exec.Command("git", "cat-file", "-t", revision)
	out, err := m.runCmdOutput(cmd, runOpts{SkipErrorLogging: true})
	if out == "commit" && err == nil {
		return true
	}
	return false
}

// Fetch fetches latest updates from origin
func (m *nativeGitClient) Fetch(revision string) error {
	if m.OnFetch != nil {
		done := m.OnFetch(m.repoURL)
		defer done()
	}

	err := m.fetch(revision)

	// When we have LFS support enabled, check for large files and fetch them too.
	if err == nil && m.IsLFSEnabled() {
		largeFiles, err := m.LsLargeFiles()
		if err == nil && len(largeFiles) > 0 {
			err = m.runCredentialedCmd("lfs", "fetch", "--all")
			if err != nil {
				return err
			}
		}
	}

	return err
}

// LsFiles lists the local working tree, including only files that are under source control
func (m *nativeGitClient) LsFiles(path string, enableNewGitFileGlobbing bool) ([]string, error) {
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
	out, err := m.runCmd("ls-files", "--full-name", "-z", "--", path)
	if err != nil {
		return nil, err
	}
	// remove last element, which is blank regardless of whether we're using nullbyte or newline
	ss := strings.Split(out, "\000")
	return ss[:len(ss)-1], nil
}

// LsLargeFiles lists all files that have references to LFS storage
func (m *nativeGitClient) LsLargeFiles() ([]string, error) {
	out, err := m.runCmd("lfs", "ls-files", "-n")
	if err != nil {
		return nil, err
	}
	ss := strings.Split(out, "\n")
	return ss, nil
}

// Submodule embed other repositories into this repository
func (m *nativeGitClient) Submodule() error {
	if err := m.runCredentialedCmd("submodule", "sync", "--recursive"); err != nil {
		return err
	}
	return m.runCredentialedCmd("submodule", "update", "--init", "--recursive")
}

// Checkout checks out the specified revision
func (m *nativeGitClient) Checkout(revision string, submoduleEnabled bool) (string, error) {
	if revision == "" || revision == "HEAD" {
		revision = "origin/HEAD"
	}
	if out, err := m.runCmd("checkout", "--force", revision); err != nil {
		return out, fmt.Errorf("failed to checkout %s: %w", revision, err)
	}
	// We must populate LFS content by using lfs checkout, if we have at least
	// one LFS reference in the current revision.
	if m.IsLFSEnabled() {
		largeFiles, err := m.LsLargeFiles()
		if err != nil {
			return "", fmt.Errorf("failed to list LFS files: %w", err)
		}
		if len(largeFiles) > 0 {
			if out, err := m.runCmd("lfs", "checkout"); err != nil {
				return out, fmt.Errorf("failed to checkout LFS files: %w", err)
			}
		}
	}
	if _, err := os.Stat(m.root + "/.gitmodules"); !os.IsNotExist(err) {
		if submoduleEnabled {
			if err := m.Submodule(); err != nil {
				return "", fmt.Errorf("failed to update submodules: %w", err)
			}
		}
	}
	// NOTE
	// The double “f” in the arguments is not a typo: the first “f” tells
	// `git clean` to delete untracked files and directories, and the second “f”
	// tells it to clean untracked nested Git repositories (for example a
	// submodule which has since been removed).
	if out, err := m.runCmd("clean", "-ffdx"); err != nil {
		return out, fmt.Errorf("failed to clean: %w", err)
	}
	return "", nil
}

func (m *nativeGitClient) getRefs() ([]*plumbing.Reference, error) {
	myLockUUID, err := uuid.NewRandom()
	myLockId := ""
	if err != nil {
		log.Debug("Error generating git references cache lock id: ", err)
	} else {
		myLockId = myLockUUID.String()
	}
	// Prevent an additional get call to cache if we know our state isn't stale
	needsUnlock := true
	if m.gitRefCache != nil && m.loadRefFromCache {
		var res []*plumbing.Reference
		foundLockId, err := m.gitRefCache.GetOrLockGitReferences(m.repoURL, myLockId, &res)
		isLockOwner := myLockId == foundLockId
		if !isLockOwner && err == nil {
			// Valid value already in cache
			return res, nil
		} else if !isLockOwner && err != nil {
			// Error getting value from cache
			log.Debugf("Error getting git references from cache: %v", err)
			return nil, err
		}
		// Defer a soft reset of the cache lock, if the value is set this call will be ignored
		defer func() {
			if needsUnlock {
				err := m.gitRefCache.UnlockGitReferences(m.repoURL, myLockId)
				if err != nil {
					log.Debugf("Error unlocking git references from cache: %v", err)
				}
			}
		}()
	}

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
	res, err := listRemote(remote, &git.ListOptions{Auth: auth}, m.insecure, m.creds, m.proxy, m.noProxy)
	if err == nil && m.gitRefCache != nil {
		if err := m.gitRefCache.SetGitReferences(m.repoURL, res); err != nil {
			log.Warnf("Failed to store git references to cache: %v", err)
		} else {
			// Since we successfully overwrote the lock with valid data, we don't need to unlock
			needsUnlock = false
		}
		return res, nil
	}
	return res, err
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
			return
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
	return
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

func (m *nativeGitClient) lsRemote(revision string) (string, error) {
	if IsCommitSHA(revision) {
		return revision, nil
	}

	refs, err := m.getRefs()
	if err != nil {
		return "", fmt.Errorf("failed to list refs: %w", err)
	}

	if revision == "" {
		revision = "HEAD"
	}

	maxV, err := versions.MaxVersion(revision, getGitTags(refs))
	if err == nil {
		revision = maxV
	}

	// refToHash keeps a maps of remote refs to their hash
	// (e.g. refs/heads/master -> a67038ae2e9cb9b9b16423702f98b41e36601001)
	refToHash := make(map[string]string)

	// refToResolve remembers ref name of the supplied revision if we determine the revision is a
	// symbolic reference (like HEAD), in which case we will resolve it from the refToHash map
	refToResolve := ""

	for _, ref := range refs {
		refName := ref.Name().String()
		hash := ref.Hash().String()
		if ref.Type() == plumbing.HashReference {
			refToHash[refName] = hash
		}
		// log.Debugf("%s\t%s", hash, refName)
		if ref.Name().Short() == revision || refName == revision {
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
	if IsTruncatedCommitSHA(revision) {
		log.Debugf("revision '%s' assumed to be commit sha", revision)
		return revision, nil
	}

	// If we get here, revision string had non hexadecimal characters (indicating its a branch, tag,
	// or symbolic ref) and we were unable to resolve it to a commit SHA.
	return "", fmt.Errorf("unable to resolve '%s' to a commit SHA", revision)
}

// CommitSHA returns current commit sha from `git rev-parse HEAD`
func (m *nativeGitClient) CommitSHA() (string, error) {
	out, err := m.runCmd("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// RevisionMetadata returns the meta-data for the commit
func (m *nativeGitClient) RevisionMetadata(revision string) (*RevisionMetadata, error) {
	out, err := m.runCmd("show", "-s", "--format=%an <%ae>%n%at%n%B", revision)
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

	cmd := exec.Command("git", "interpret-trailers", "--parse")
	cmd.Stdin = strings.NewReader(message)
	out, err = m.runCmdOutput(cmd, runOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to interpret trailers for revision %q in repo %q: %w", revision, m.repoURL, err)
	}
	relatedCommits := getReferences(log.WithFields(log.Fields{"repo": m.repoURL, "revision": revision}), out)

	out, err = m.runCmd("tag", "--points-at", revision)
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

// getReferences extracts related commit metadata from the commit message trailers. If referenced commit
// metadata is present, we return a slice containing a single metadata object. If no related commit metadata is found,
// we return a nil slice.
//
// If a trailer fails validation, we log an error and skip that trailer. We truncate the trailer values to 100
// characters to avoid excessively long log messages.
func getReferences(logCtx *log.Entry, commitMessageBody string) []RevisionReference {
	var relatedCommit CommitMetadata
	scanner := bufio.NewScanner(strings.NewReader(commitMessageBody))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Argocd-reference-commit-") {
			continue
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		trailerKey := parts[0]
		trailerValue := parts[1]
		switch trailerKey {
		case "Argocd-reference-commit-repourl":
			_, err := url.Parse(trailerValue)
			if err != nil {
				logCtx.Errorf("failed to parse repo URL %q: %v", truncate(trailerValue), err)
				continue
			}
			relatedCommit.RepoURL = trailerValue
		case "Argocd-reference-commit-author":
			address, err := mail.ParseAddress(trailerValue)
			if err != nil || address == nil {
				logCtx.Errorf("failed to parse author email %q: %v", truncate(trailerValue), err)
				continue
			}
			relatedCommit.Author = *address
		case "Argocd-reference-commit-date":
			// Validate that it's the correct date format.
			t, err := time.Parse(time.RFC3339, trailerValue)
			if err != nil {
				logCtx.Errorf("failed to parse date %q with RFC3339 format: %v", truncate(trailerValue), err)
				continue
			}
			relatedCommit.Date = t.Format(time.RFC3339)
		case "Argocd-reference-commit-subject":
			relatedCommit.Subject = trailerValue
		case "Argocd-reference-commit-body":
			body := ""
			err := json.Unmarshal([]byte(trailerValue), &body)
			if err != nil {
				logCtx.Errorf("failed to parse body %q as JSON: %v", truncate(trailerValue), err)
				continue
			}
			relatedCommit.Body = body
		case "Argocd-reference-commit-sha":
			if !shaRegex.MatchString(trailerValue) {
				logCtx.Errorf("invalid commit SHA %q in trailer %s: must be a lowercase hex string 5-40 characters long", truncate(trailerValue), trailerKey)
				continue
			}
			relatedCommit.SHA = trailerValue
		}
	}
	var relatedCommits []RevisionReference
	if relatedCommit != (CommitMetadata{}) {
		relatedCommits = append(relatedCommits, RevisionReference{
			Commit: &relatedCommit,
		})
	}
	return relatedCommits
}

// VerifyCommitSignature Runs verify-commit on a given revision and returns the output
func (m *nativeGitClient) VerifyCommitSignature(revision string) (string, error) {
	out, err := m.runGnuPGWrapper("git-verify-wrapper.sh", revision)
	if err != nil {
		log.Errorf("error verifying commit signature: %v", err)
		return "", errors.New("permission denied")
	}
	return out, nil
}

// IsAnnotatedTag returns true if the revision points to an annotated tag
func (m *nativeGitClient) IsAnnotatedTag(revision string) bool {
	cmd := exec.Command("git", "describe", "--exact-match", revision)
	out, err := m.runCmdOutput(cmd, runOpts{SkipErrorLogging: true})
	if out != "" && err == nil {
		return true
	}
	return false
}

// ChangedFiles returns a list of files changed between two revisions
func (m *nativeGitClient) ChangedFiles(revision string, targetRevision string) ([]string, error) {
	if revision == targetRevision {
		return []string{}, nil
	}

	if !IsCommitSHA(revision) || !IsCommitSHA(targetRevision) {
		return []string{}, errors.New("invalid revision provided, must be SHA")
	}

	out, err := m.runCmd("diff", "--name-only", fmt.Sprintf("%s..%s", revision, targetRevision))
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
func (m *nativeGitClient) config(args ...string) (string, error) {
	args = append([]string{"config"}, args...)
	out, err := m.runCmd(args...)
	if err != nil {
		return out, fmt.Errorf("failed to run git config: %w", err)
	}
	return out, nil
}

// SetAuthor sets the author name and email in the git configuration.
func (m *nativeGitClient) SetAuthor(name, email string) (string, error) {
	if name != "" {
		out, err := m.config("--local", "user.name", name)
		if err != nil {
			return out, err
		}
	}
	if email != "" {
		out, err := m.config("--local", "user.email", email)
		if err != nil {
			return out, err
		}
	}
	return "", nil
}

// CheckoutOrOrphan checks out the branch. If the branch does not exist, it creates an orphan branch.
func (m *nativeGitClient) CheckoutOrOrphan(branch string, submoduleEnabled bool) (string, error) {
	out, err := m.Checkout(branch, submoduleEnabled)
	if err != nil {
		// If the branch doesn't exist, create it as an orphan branch.
		if !strings.Contains(err.Error(), "did not match any file(s) known to git") {
			return out, fmt.Errorf("failed to checkout branch: %w", err)
		}
		out, err = m.runCmd("switch", "--orphan", branch)
		if err != nil {
			return out, fmt.Errorf("failed to create orphan branch: %w", err)
		}

		// Make an empty initial commit.
		out, err = m.runCmd("commit", "--allow-empty", "-m", "Initial commit")
		if err != nil {
			return out, fmt.Errorf("failed to commit initial commit: %w", err)
		}

		// Push the commit.
		err = m.runCredentialedCmd("push", "origin", branch)
		if err != nil {
			return "", fmt.Errorf("failed to push to branch: %w", err)
		}
	}
	return "", nil
}

// CheckoutOrNew checks out the given branch. If the branch does not exist, it creates an empty branch based on
// the base branch.
func (m *nativeGitClient) CheckoutOrNew(branch, base string, submoduleEnabled bool) (string, error) {
	out, err := m.Checkout(branch, submoduleEnabled)
	if err != nil {
		if !strings.Contains(err.Error(), "did not match any file(s) known to git") {
			return out, fmt.Errorf("failed to checkout branch: %w", err)
		}
		// If the branch does not exist, create any empty branch based on the sync branch
		// First, checkout the sync branch.
		out, err = m.Checkout(base, submoduleEnabled)
		if err != nil {
			return out, fmt.Errorf("failed to checkout sync branch: %w", err)
		}

		out, err = m.runCmd("checkout", "-b", branch)
		if err != nil {
			return out, fmt.Errorf("failed to create branch: %w", err)
		}
	}
	return "", nil
}

// RemoveContents removes all files from the git repository.
func (m *nativeGitClient) RemoveContents() (string, error) {
	out, err := m.runCmd("rm", "-r", "--ignore-unmatch", ".")
	if err != nil {
		return out, fmt.Errorf("failed to clear repo contents: %w", err)
	}
	return "", nil
}

// CommitAndPush commits and pushes changes to the target branch.
func (m *nativeGitClient) CommitAndPush(branch, message string) (string, error) {
	out, err := m.runCmd("add", ".")
	if err != nil {
		return out, fmt.Errorf("failed to add files: %w", err)
	}

	out, err = m.runCmd("commit", "-m", message)
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

	err = m.runCredentialedCmd("push", "origin", branch)
	if err != nil {
		return "", fmt.Errorf("failed to push: %w", err)
	}

	return "", nil
}

// runWrapper runs a custom command with all the semantics of running the Git client
func (m *nativeGitClient) runGnuPGWrapper(wrapper string, args ...string) (string, error) {
	cmd := exec.Command(wrapper, args...)
	cmd.Env = append(cmd.Env, "GNUPGHOME="+common.GetGnuPGHomePath(), "LANG=C")
	return m.runCmdOutput(cmd, runOpts{})
}

// runCmd is a convenience function to run a command in a given directory and return its output
func (m *nativeGitClient) runCmd(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	return m.runCmdOutput(cmd, runOpts{})
}

// runCredentialedCmd is a convenience function to run a git command with username/password credentials
func (m *nativeGitClient) runCredentialedCmd(args ...string) error {
	closer, environ, err := m.creds.Environ()
	if err != nil {
		return err
	}
	defer func() { _ = closer.Close() }()

	// If a basic auth header is explicitly set, tell Git to send it to the
	// server to force use of basic auth instead of negotiating the auth scheme
	for _, e := range environ {
		if strings.HasPrefix(e, forceBasicAuthHeaderEnv+"=") {
			args = append([]string{"--config-env", "http.extraHeader=" + forceBasicAuthHeaderEnv}, args...)
		} else if strings.HasPrefix(e, bearerAuthHeaderEnv+"=") {
			args = append([]string{"--config-env", "http.extraHeader=" + bearerAuthHeaderEnv}, args...)
		}
	}

	cmd := exec.Command("git", args...)
	cmd.Env = append(cmd.Env, environ...)
	_, err = m.runCmdOutput(cmd, runOpts{})
	return err
}

func (m *nativeGitClient) runCmdOutput(cmd *exec.Cmd, ropts runOpts) (string, error) {
	cmd.Dir = m.root
	cmd.Env = append(os.Environ(), cmd.Env...)
	// Set $HOME to nowhere, so we can execute Git regardless of any external
	// authentication keys (e.g. in ~/.ssh) -- this is especially important for
	// running tests on local machines and/or CircleCI.
	cmd.Env = append(cmd.Env, "HOME=/dev/null")
	// Skip LFS for most Git operations except when explicitly requested
	cmd.Env = append(cmd.Env, "GIT_LFS_SKIP_SMUDGE=1")
	// Disable Git terminal prompts in case we're running with a tty
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=false")

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
