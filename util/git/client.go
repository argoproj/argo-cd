package git

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	ssh2 "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

// Client is a generic git client interface
type Client interface {
	Root() string
	Init() error
	Fetch() error
	Checkout(revision string) error
	LsRemote(revision string) (string, error)
	LsFiles(path string) ([]string, error)
	CommitSHA() (string, error)
}

// ClientFactory is a factory of Git Clients
// Primarily used to support creation of mock git clients during unit testing
type ClientFactory interface {
	NewClient(repoURL, path, username, password, sshPrivateKey string, insecureIgnoreHostKey bool) (Client, error)
}

// nativeGitClient implements Client interface using git CLI
type nativeGitClient struct {
	repoURL string
	root    string
	auth    transport.AuthMethod
}

type factory struct{}

func NewFactory() ClientFactory {
	return &factory{}
}

func (f *factory) NewClient(rawRepoURL, path, username, password, sshPrivateKey string, insecureIgnoreHostKey bool) (Client, error) {
	var repoUser string

	if repoURL, err := url.Parse(rawRepoURL); err != nil {
		return nil, err
	} else {
		repoUser = repoURL.User.Username()
	}

	clnt := nativeGitClient{
		repoURL: rawRepoURL,
		root:    path,
	}
	if sshPrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(sshPrivateKey))
		if err != nil {
			return nil, err
		}
		auth := &ssh2.PublicKeys{User: repoUser, Signer: signer}
		if insecureIgnoreHostKey {
			auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
		clnt.auth = auth
	} else if username != "" || password != "" {
		auth := &http.BasicAuth{Username: username, Password: password}
		clnt.auth = auth
	}
	return &clnt, nil
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
	_, err = exec.Command("rm", "-rf", m.root).Output()
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
	log.Debugf("Fetching repo %s at %s", m.repoURL, m.root)
	// Two techniques are used for fetching the remote depending if the remote is SSH vs. HTTPS
	// If http, we fork/exec the git CLI since the go-git client does not properly support git
	// providers such as AWS CodeCommit and Azure DevOps.
	if _, ok := m.auth.(*ssh2.PublicKeys); ok {
		return m.goGitFetch()
	}
	_, err := m.runCredentialedCmd("git", "fetch", "origin", "--tags", "--force")
	return err
}

// goGitFetch fetches the remote using go-git
func (m *nativeGitClient) goGitFetch() error {
	repo, err := git.PlainOpen(m.root)
	if err != nil {
		return err
	}

	log.Debug("git fetch origin --tags --force")
	err = repo.Fetch(&git.FetchOptions{
		RemoteName: git.DefaultRemoteName,
		Auth:       m.auth,
		Tags:       git.AllTags,
		Force:      true,
	})
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

// LsFiles lists the local working tree, including only files that are under source control
func (m *nativeGitClient) LsFiles(path string) ([]string, error) {
	out, err := m.runCmd("git", "ls-files", "--full-name", "-z", "--", path)
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
	if _, err := m.runCmd("git", "checkout", "--force", revision); err != nil {
		return err
	}
	if _, err := m.runCmd("git", "clean", "-fdx"); err != nil {
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
	refs, err := remote.List(&git.ListOptions{Auth: m.auth})
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
	out, err := m.runCmd("git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// runCmd is a convenience function to run a command in a given directory and return its output
func (m *nativeGitClient) runCmd(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	return m.runCmdOutput(cmd)
}

// runCredentialedCmd is a convenience function to run a git command with username/password credentials
func (m *nativeGitClient) runCredentialedCmd(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	if auth, ok := m.auth.(*http.BasicAuth); ok {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_ASKPASS=git-ask-pass.sh"))
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_USERNAME=%s", auth.Username))
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_PASSWORD=%s", auth.Password))
	}
	return m.runCmdOutput(cmd)
}

func (m *nativeGitClient) runCmdOutput(cmd *exec.Cmd) (string, error) {
	log.Debug(strings.Join(cmd.Args, " "))
	cmd.Dir = m.root
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, "HOME=/dev/null")
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=true")
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOGLOBAL=true")
	out, err := cmd.Output()
	if len(out) > 0 {
		log.Debug(string(out))
	}
	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if ok {
			errOutput := strings.Split(string(exErr.Stderr), "\n")[0]
			log.Debug(errOutput)
			return string(out), fmt.Errorf("'%s' failed: %v", strings.Join(cmd.Args, " "), errOutput)
		}
		return string(out), fmt.Errorf("'%s' failed: %v", strings.Join(cmd.Args, " "), err)
	}
	return string(out), nil
}
