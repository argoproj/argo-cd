package git

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	argoexec "github.com/argoproj/pkg/exec"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	ssh2 "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"gopkg.in/src-d/go-git.v4/storage/memory"

	certutil "github.com/argoproj/argo-cd/util/cert"
	argoconfig "github.com/argoproj/argo-cd/util/config"
)

type RevisionMetadata struct {
	Author  string
	Date    time.Time
	Tags    []string
	Message string
}

// Client is a generic git client interface
type Client interface {
	Root() string
	Init() error
	Fetch() error
	Checkout(revision string) error
	LsRemote(revision string) (string, error)
	LsFiles(path string) ([]string, error)
	CommitSHA() (string, error)
	RevisionMetadata(revision string) (*RevisionMetadata, error)
}

// ClientFactory is a factory of Git Clients
// Primarily used to support creation of mock git clients during unit testing
type ClientFactory interface {
	NewClient(repoURL, path, username, password, sshPrivateKey string, insecure bool) (Client, error)
}

// nativeGitClient implements Client interface using git CLI
type nativeGitClient struct {
	// URL of the repository
	repoURL string
	// Root path of repository
	root string
	// Authenticator credentials for private repositories
	creds Creds
	// Whether to connect insecurely to repository, e.g. don't verify certificate
	insecure bool
}

type factory struct{}

func NewFactory() ClientFactory {
	return &factory{}
}

func (f *factory) NewClient(rawRepoURL, path, username, password, sshPrivateKey string, insecure bool) (Client, error) {
	var creds Creds
	if sshPrivateKey != "" {
		creds = SSHCreds{sshPrivateKey, insecure}
	} else if username != "" || password != "" {
		creds = HTTPSCreds{username, password}
	} else {
		creds = NopCreds{}
	}

	// We need a custom HTTP client for go-git when we want to skip validation
	// of the server's TLS certificate (--insecure-ignore-server-cert). Since
	// this change is permanent to go-git Client during runtime, we need to
	// explicitly replace it with default client for repositories without the
	// insecure flag set.
	//if IsHTTPSURL(rawRepoURL) {
	//	gitclient.InstallProtocol("https", githttp.NewClient(getRepoHTTPClient(rawRepoURL, insecure)))
	//}
	client := nativeGitClient{
		repoURL:  rawRepoURL,
		root:     path,
		creds:    creds,
		insecure: insecure,
	}
	return &client, nil
}

// Returns a HTTP client object suitable for go-git to use using the following
// pattern:
// - If insecure is true, always returns a client with certificate verification
//   turned off.
// - If one or more custom certificates are stored for the repository, returns
//   a client with those certificates in the list of root CAs used to verify
//   the server's certificate.
// - Otherwise (and on non-fatal errors), a default HTTP client is returned.
func getRepoHTTPClient(repoURL string, insecure bool) transport.Transport {
	// Default HTTP client
	var customHTTPClient transport.Transport = githttp.NewClient(&http.Client{})

	if insecure {
		customHTTPClient = githttp.NewClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			// 15 second timeout
			Timeout: 15 * time.Second,

			// don't follow redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		})
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
			customHTTPClient = githttp.NewClient(&http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs: certPool,
					},
				},
				// 15 second timeout
				Timeout: 15 * time.Second,
				// don't follow redirect
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			})
		}
		// else no custom certificate stored.
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
		auth := &ssh2.PublicKeys{User: sshUser, Signer: signer}
		if creds.insecureIgnoreHostKey {
			auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
		return auth, nil
	case HTTPSCreds:
		auth := githttp.BasicAuth{Username: creds.username, Password: creds.password}
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
	_, err = argoexec.RunCommand("rm", argoconfig.CmdOpts(), "-rf", m.root)
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

// Fetch fetches latest updates from origin
func (m *nativeGitClient) Fetch() error {
	_, err := m.runCredentialedCmd("git", "fetch", "origin", "--tags", "--force")
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

// Checkout checkout specified git sha
func (m *nativeGitClient) Checkout(revision string) error {
	if revision == "" || revision == "HEAD" {
		revision = "origin/HEAD"
	}
	if _, err := m.runCmd("checkout", "--force", revision); err != nil {
		return err
	}
	if _, err := m.runCmd("clean", "-fdx"); err != nil {
		return err
	}
	return nil
}

// LsRemote resolves the commit SHA of a specific branch, tag, or HEAD. If the supplied revision
// does not resolve, and "looks" like a 7+ hexadecimal commit SHA, it return the revision string.
// Otherwise, it returns an error indicating that the revision could not be resolved. This method
// runs with in-memory storage and is safe to run concurrently, or to be run without a git
// repository locally cloned.
func (m *nativeGitClient) LsRemote(revision string) (string, error) {
	if IsCommitSHA(revision) {
		return revision, nil
	}
	repo, err := git.Init(memory.NewStorage(), nil)
	if err != nil {
		return "", err
	}
	remote, err := repo.CreateRemote(&config.RemoteConfig{
		Name: git.DefaultRemoteName,
		URLs: []string{m.repoURL},
	})
	if err != nil {
		return "", err
	}
	auth, err := newAuth(m.repoURL, m.creds)
	if err != nil {
		return "", err
	}
	//refs, err := remote.List(&git.ListOptions{Auth: auth})
	refs, err := listRemote(remote, &git.ListOptions{Auth: auth}, m.insecure)
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
		if refName != "HEAD" && !strings.HasPrefix(refName, "refs/heads/") && !strings.HasPrefix(refName, "refs/tags/") {
			// ignore things like 'refs/pull/' 'refs/reviewable'
			continue
		}
		hash := ref.Hash().String()
		if ref.Type() == plumbing.HashReference {
			refToHash[refName] = hash
		}
		//log.Debugf("%s\t%s", hash, refName)
		if ref.Name().Short() == revision {
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

// runCmd is a convenience function to run a command in a given directory and return its output
func (m *nativeGitClient) runCmd(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	return m.runCmdOutput(cmd)
}

// runCredentialedCmd is a convenience function to run a git command with username/password credentials
func (m *nativeGitClient) runCredentialedCmd(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	closer, environ, err := m.creds.Environ()
	if err != nil {
		return "", err
	}
	defer func() { _ = closer.Close() }()
	cmd.Env = append(cmd.Env, environ...)
	return m.runCmdOutput(cmd)
}

func (m *nativeGitClient) runCmdOutput(cmd *exec.Cmd) (string, error) {
	cmd.Dir = m.root
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, "HOME=/dev/null")
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=true")
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOGLOBAL=true")
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
	log.Debug(strings.Join(cmd.Args, " "))
	return argoexec.RunCommandExt(cmd, argoconfig.CmdOpts())
}
