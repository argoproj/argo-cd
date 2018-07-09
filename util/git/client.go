package git

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
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
	Reset() error
}

// ClientFactory is a factory of Git Clients
// Primarily used to support creation of mock git clients during unit testing
type ClientFactory interface {
	NewClient(repoURL, path, username, password, sshPrivateKey string) Client
}

// nativeGitClient implements Client interface using git CLI
type nativeGitClient struct {
	repoURL       string
	root          string
	username      string
	password      string
	sshPrivateKey string
}

type factory struct{}

func NewFactory() ClientFactory {
	return &factory{}
}

func (f *factory) NewClient(repoURL, path, username, password, sshPrivateKey string) Client {
	return &nativeGitClient{
		repoURL:       repoURL,
		root:          path,
		username:      username,
		password:      password,
		sshPrivateKey: sshPrivateKey,
	}
}

func (m *nativeGitClient) Root() string {
	return m.root
}

// Init initializes a local git repository and sets the remote origin
func (m *nativeGitClient) Init() error {
	var needInit bool
	if _, err := os.Stat(m.root); os.IsNotExist(err) {
		needInit = true
	} else {
		_, err = m.runCmd("git", "status")
		needInit = err != nil
	}
	if needInit {
		log.Infof("Initializing %s to %s", m.repoURL, m.root)
		_, err := exec.Command("rm", "-rf", m.root).Output()
		if err != nil {
			return fmt.Errorf("unable to clean repo at %s: %v", m.root, err)
		}
		err = os.MkdirAll(m.root, 0755)
		if err != nil {
			return err
		}
		if _, err := m.runCmd("git", "init"); err != nil {
			return err
		}
		if _, err := m.runCmd("git", "remote", "add", "origin", m.repoURL); err != nil {
			return err
		}
	}
	// always set credentials since it can change
	err := m.setCredentials()
	if err != nil {
		return err
	}
	return nil
}

// setCredentials sets a local credentials file to connect to a remote git repository
func (m *nativeGitClient) setCredentials() error {
	if m.password != "" {
		log.Debug("Setting password credentials")
		gitCredentialsFile := path.Join(m.root, ".git", "credentials")
		urlObj, err := url.ParseRequestURI(m.repoURL)
		if err != nil {
			return err
		}
		urlObj.User = url.UserPassword(m.username, m.password)
		cmdURL := urlObj.String()
		err = ioutil.WriteFile(gitCredentialsFile, []byte(cmdURL), 0600)
		if err != nil {
			return fmt.Errorf("failed to set git credentials: %v", err)
		}
		_, err = m.runCmd("git", "config", "--local", "credential.helper", fmt.Sprintf("store --file=%s", gitCredentialsFile))
		if err != nil {
			return err
		}
	}
	if IsSSHURL(m.repoURL) {
		sshCmd := gitSSHCommand
		if m.sshPrivateKey != "" {
			log.Debug("Setting SSH credentials")
			sshPrivateKeyFile := path.Join(".git", "ssh-private-key")
			err := ioutil.WriteFile(sshPrivateKeyFile, []byte(m.sshPrivateKey), 0600)
			if err != nil {
				return fmt.Errorf("failed to set git credentials: %v", err)
			}
			sshCmd += sshCmd + " -i " + sshPrivateKeyFile
		}
		_, err := m.runCmd("git", "config", "--local", "core.sshCommand", sshCmd)
		if err != nil {
			return err
		}
	}
	return nil
}

// Fetch fetches latest updates from origin
func (m *nativeGitClient) Fetch() error {
	var err error
	log.Debugf("Fetching repo %s at %s", m.repoURL, m.root)
	if _, err = m.runCmd("git", "fetch", "origin"); err != nil {
		return err
	}
	// git fetch does not update the HEAD reference. The following command will update the local
	// knowledge of what remote considers the “default branch”
	// See: https://stackoverflow.com/questions/8839958/how-does-origin-head-get-set
	if _, err := m.runCmd("git", "remote", "set-head", "origin", "-a"); err != nil {
		return err
	}
	return nil
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

// Reset resets local changes in a repository
func (m *nativeGitClient) Reset() error {
	if _, err := m.runCmd("git", "reset", "--hard", "origin/HEAD"); err != nil {
		return err
	}
	// Delete all local branches (we must first detach so we are not checked out a branch we are about to delete)
	if _, err := m.runCmd("git", "checkout", "--detach", "origin/HEAD"); err != nil {
		return err
	}
	branchesOut, err := m.runCmd("git", "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return err
	}
	branchesOut = strings.TrimSpace(branchesOut)
	if branchesOut != "" {
		branches := strings.Split(branchesOut, "\n")
		args := []string{"branch", "-D"}
		args = append(args, branches...)
		if _, err = m.runCmd("git", args...); err != nil {
			return err
		}
	}
	if _, err := m.runCmd("git", "clean", "-fd"); err != nil {
		return err
	}
	return nil
}

// Checkout checkout specified git sha
func (m *nativeGitClient) Checkout(revision string) error {
	if revision == "" || revision == "HEAD" {
		revision = "origin/HEAD"
	}
	if _, err := m.runCmd("git", "checkout", revision); err != nil {
		return err
	}
	return nil
}

// LsRemote returns the commit SHA of a specific branch, tag, or HEAD
func (m *nativeGitClient) LsRemote(revision string) (string, error) {
	var args []string
	if revision == "" || revision == "HEAD" {
		args = []string{"ls-remote", "origin", "HEAD"}
	} else {
		args = []string{"ls-remote", "--head", "--tags", "origin", revision}

	}
	out, err := m.runCmd("git", args...)
	if err != nil {
		return "", err
	}
	if out == "" {
		// if doesn't exist in remote, assume revision is a commit sha and return it
		return revision, nil
	}
	// 3f4ec0ab2263038ba91d3b594b2188fc108fc8d7	refs/heads/master
	return strings.Fields(out)[0], nil
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
	log.Debug(strings.Join(cmd.Args, " "))
	cmd.Dir = m.root
	env := os.Environ()
	env = append(env, "GIT_ASKPASS=")
	cmd.Env = env
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
