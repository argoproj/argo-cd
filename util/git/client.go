package git

import (
	"crypto/tls"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"

	"github.com/argoproj/argo-cd/v2/common"
	certutil "github.com/argoproj/argo-cd/v2/util/cert"
	"github.com/argoproj/argo-cd/v2/util/env"
	executil "github.com/argoproj/argo-cd/v2/util/exec"
	"github.com/argoproj/argo-cd/v2/util/proxy"
)

type RevisionMetadata struct {
	Author  string
	Date    time.Time
	Tags    []string
	Message string
}

// this should match reposerver/repository/repository.proto/RefsList
type Refs struct {
	Branches []string
	Tags     []string
	// heads and remotes are also refs, but are not needed at this time.
}

type gitRefCache interface {
	SetGitReferences(repo string, references []*plumbing.Reference) error
	GetGitReferences(repo string, references *[]*plumbing.Reference) error
}

// Client is a generic git client interface
type Client interface {
	Root() string
	Init() error
	Fetch(revision string) error
	Checkout(revision string, submoduleEnabled bool) error
	LsRefs() (*Refs, error)
	LsRemote(revision string) (string, error)
	LsFiles(path string) ([]string, error)
	LsLargeFiles() ([]string, error)
	CommitSHA() (string, error)
	RevisionMetadata(revision string) (*RevisionMetadata, error)
	VerifyCommitSignature(string) (string, error)
}

type EventHandlers struct {
	OnLsRemote func(repo string) func()
	OnFetch    func(repo string) func()
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
}

var (
	maxAttemptsCount = 1
	maxRetryDuration time.Duration
	retryDuration    time.Duration
	factor           int64
)

func init() {
	if countStr := os.Getenv(common.EnvGitAttemptsCount); countStr != "" {
		if cnt, err := strconv.Atoi(countStr); err != nil {
			panic(fmt.Sprintf("Invalid value in %s env variable: %v", common.EnvGitAttemptsCount, err))
		} else {
			maxAttemptsCount = int(math.Max(float64(cnt), 1))
		}
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

func NewClient(rawRepoURL string, creds Creds, insecure bool, enableLfs bool, proxy string, opts ...ClientOpts) (Client, error) {
	r := regexp.MustCompile("(/|:)")
	root := filepath.Join(os.TempDir(), r.ReplaceAllString(NormalizeGitURL(rawRepoURL), "_"))
	if root == os.TempDir() {
		return nil, fmt.Errorf("Repository '%s' cannot be initialized, because its root would be system temp at %s", rawRepoURL, root)
	}
	return NewClientExt(rawRepoURL, root, creds, insecure, enableLfs, proxy, opts...)
}

func NewClientExt(rawRepoURL string, root string, creds Creds, insecure bool, enableLfs bool, proxy string, opts ...ClientOpts) (Client, error) {
	client := &nativeGitClient{
		repoURL:   rawRepoURL,
		root:      root,
		creds:     creds,
		insecure:  insecure,
		enableLfs: enableLfs,
		proxy:     proxy,
	}
	for i := range opts {
		opts[i](client)
	}
	return client, nil
}

// Returns a HTTP client object suitable for go-git to use using the following
// pattern:
// - If insecure is true, always returns a client with certificate verification
//   turned off.
// - If one or more custom certificates are stored for the repository, returns
//   a client with those certificates in the list of root CAs used to verify
//   the server's certificate.
// - Otherwise (and on non-fatal errors), a default HTTP client is returned.
func GetRepoHTTPClient(repoURL string, insecure bool, creds Creds, proxyURL string) *http.Client {
	// Default HTTP client
	var customHTTPClient = &http.Client{
		// 15 second timeout
		Timeout: 15 * time.Second,
		// don't follow redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	proxyFunc := proxy.GetCallback(proxyURL)

	// Callback function to return any configured client certificate
	// We never return err, but an empty cert instead.
	clientCertFunc := func(req *tls.CertificateRequestInfo) (*tls.Certificate, error) {
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

	if insecure {
		customHTTPClient.Transport = &http.Transport{
			Proxy: proxyFunc,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify:   true,
				GetClientCertificate: clientCertFunc,
			},
			DisableKeepAlives: true,
		}
	} else {
		parsedURL, err := url.Parse(repoURL)
		if err != nil {
			return customHTTPClient
		}
		serverCertificatePem, err := certutil.GetCertificateForConnect(parsedURL.Host)
		if err != nil {
			return customHTTPClient
		} else if len(serverCertificatePem) > 0 {
			certPool := certutil.GetCertPoolFromPEMData(serverCertificatePem)
			customHTTPClient.Transport = &http.Transport{
				Proxy: proxyFunc,
				TLSClientConfig: &tls.Config{
					RootCAs:              certPool,
					GetClientCertificate: clientCertFunc,
				},
				DisableKeepAlives: true,
			}
		} else {
			// else no custom certificate stored.
			customHTTPClient.Transport = &http.Transport{
				Proxy: proxyFunc,
				TLSClientConfig: &tls.Config{
					GetClientCertificate: clientCertFunc,
				},
				DisableKeepAlives: true,
			}
		}
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
	if err != git.ErrRepositoryNotExists {
		return err
	}
	log.Infof("Initializing %s to %s", m.repoURL, m.root)
	_, err = executil.Run(exec.Command("rm", "-rf", m.root))
	if err != nil {
		return fmt.Errorf("unable to clean repo at %s: %v", m.root, err)
	}
	err = os.MkdirAll(m.root, 0755)
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

// Returns true if the repository is LFS enabled
func (m *nativeGitClient) IsLFSEnabled() bool {
	return m.enableLfs
}

// Fetch fetches latest updates from origin
func (m *nativeGitClient) Fetch(revision string) error {
	if m.OnFetch != nil {
		done := m.OnFetch(m.repoURL)
		defer done()
	}

	var err error
	if revision != "" {
		err = m.runCredentialedCmd("git", "fetch", "origin", revision, "--tags", "--force")
	} else {
		err = m.runCredentialedCmd("git", "fetch", "origin", "--tags", "--force")
	}
	// When we have LFS support enabled, check for large files and fetch them too.
	if err == nil && m.IsLFSEnabled() {
		largeFiles, err := m.LsLargeFiles()
		if err == nil && len(largeFiles) > 0 {
			err = m.runCredentialedCmd("git", "lfs", "fetch", "--all")
			if err != nil {
				return err
			}
		}
	}
	return err
}

// LsFiles lists the local working tree, including only files that are under source control
func (m *nativeGitClient) LsFiles(path string) ([]string, error) {
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

// Checkout checkout specified revision
func (m *nativeGitClient) Checkout(revision string, submoduleEnabled bool) error {
	if revision == "" || revision == "HEAD" {
		revision = "origin/HEAD"
	}
	if _, err := m.runCmd("checkout", "--force", revision); err != nil {
		return err
	}
	// We must populate LFS content by using lfs checkout, if we have at least
	// one LFS reference in the current revision.
	if m.IsLFSEnabled() {
		if largeFiles, err := m.LsLargeFiles(); err == nil {
			if len(largeFiles) > 0 {
				if _, err := m.runCmd("lfs", "checkout"); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	if _, err := os.Stat(m.root + "/.gitmodules"); !os.IsNotExist(err) {
		if submoduleEnabled {
			if err := m.runCredentialedCmd("git", "submodule", "update", "--init", "--recursive"); err != nil {
				return err
			}
		}
	}
	if _, err := m.runCmd("clean", "-fdx"); err != nil {
		return err
	}
	return nil
}

func (m *nativeGitClient) getRefs() ([]*plumbing.Reference, error) {
	if m.gitRefCache != nil && m.loadRefFromCache {
		var res []*plumbing.Reference
		if m.gitRefCache.GetGitReferences(m.repoURL, &res) == nil {
			return res, nil
		}
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
	res, err := listRemote(remote, &git.ListOptions{Auth: auth}, m.insecure, m.creds, m.proxy)
	if err == nil && m.gitRefCache != nil {
		if err := m.gitRefCache.SetGitReferences(m.repoURL, res); err != nil {
			log.Warnf("Failed to store git references to cache: %v", err)
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

// LsRemote resolves the commit SHA of a specific branch, tag, or HEAD. If the supplied revision
// does not resolve, and "looks" like a 7+ hexadecimal commit SHA, it return the revision string.
// Otherwise, it returns an error indicating that the revision could not be resolved. This method
// runs with in-memory storage and is safe to run concurrently, or to be run without a git
// repository locally cloned.
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

func (m *nativeGitClient) lsRemote(revision string) (string, error) {
	if IsCommitSHA(revision) {
		return revision, nil
	}

	refs, err := m.getRefs()

	if err != nil {
		return "", err
	}
	if revision == "" {
		revision = "HEAD"
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
		//log.Debugf("%s\t%s", hash, refName)
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
	return "", fmt.Errorf("Unable to resolve '%s' to a commit SHA", revision)
}

// CommitSHA returns current commit sha from `git rev-parse HEAD`
func (m *nativeGitClient) CommitSHA() (string, error) {
	out, err := m.runCmd("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// returns the meta-data for the commit
func (m *nativeGitClient) RevisionMetadata(revision string) (*RevisionMetadata, error) {
	out, err := m.runCmd("show", "-s", "--format=%an <%ae>|%at|%B", revision)
	if err != nil {
		return nil, err
	}
	segments := strings.SplitN(out, "|", 3)
	if len(segments) != 3 {
		return nil, fmt.Errorf("expected 3 segments, got %v", segments)
	}
	author := segments[0]
	authorDateUnixTimestamp, _ := strconv.ParseInt(segments[1], 10, 64)
	message := strings.TrimSpace(segments[2])

	out, err = m.runCmd("tag", "--points-at", revision)
	if err != nil {
		return nil, err
	}
	tags := strings.Fields(out)

	return &RevisionMetadata{author, time.Unix(authorDateUnixTimestamp, 0), tags, message}, nil
}

// VerifyCommitSignature Runs verify-commit on a given revision and returns the output
func (m *nativeGitClient) VerifyCommitSignature(revision string) (string, error) {
	out, err := m.runGnuPGWrapper("git-verify-wrapper.sh", revision)
	if err != nil {
		return "", err
	}
	return out, nil
}

// runWrapper runs a custom command with all the semantics of running the Git client
func (m *nativeGitClient) runGnuPGWrapper(wrapper string, args ...string) (string, error) {
	cmd := exec.Command(wrapper, args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("GNUPGHOME=%s", common.GetGnuPGHomePath()), "LANG=C")
	return m.runCmdOutput(cmd)
}

// runCmd is a convenience function to run a command in a given directory and return its output
func (m *nativeGitClient) runCmd(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	return m.runCmdOutput(cmd)
}

// runCredentialedCmd is a convenience function to run a git command with username/password credentials
// nolint:unparam
func (m *nativeGitClient) runCredentialedCmd(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	closer, environ, err := m.creds.Environ()
	if err != nil {
		return err
	}
	defer func() { _ = closer.Close() }()
	cmd.Env = append(cmd.Env, environ...)
	_, err = m.runCmdOutput(cmd)
	return err
}

func (m *nativeGitClient) runCmdOutput(cmd *exec.Cmd) (string, error) {
	cmd.Dir = m.root
	cmd.Env = append(os.Environ(), cmd.Env...)
	// Set $HOME to nowhere, so we can be execute Git regardless of any external
	// authentication keys (e.g. in ~/.ssh) -- this is especially important for
	// running tests on local machines and/or CircleCI.
	cmd.Env = append(cmd.Env, "HOME=/dev/null")
	// Skip LFS for most Git operations except when explicitly requested
	cmd.Env = append(cmd.Env, "GIT_LFS_SKIP_SMUDGE=1")

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
					cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_SSL_CAINFO=%s", caPath))
				}
			}
		}
	}

	cmd.Env = proxy.UpsertEnv(cmd, m.proxy)

	return executil.Run(cmd)
}
