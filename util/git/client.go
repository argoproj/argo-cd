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
	CloneOrFetch(url string, username string, password string, sshPrivateKey string, repoPath string) error
	Checkout(repoPath string, sha string) (string, error)
	CommitSHA(repoPath string) (string, error)
	Reset(repoPath string) error
}

// NativeGitClient implements Client interface using git CLI
type NativeGitClient struct{}

// Init initializes a local git repository and sets the remote origin
func (m *NativeGitClient) Init(repo string, repoPath string) error {
	log.Infof("Initializing %s to %s", repo, repoPath)
	err := os.MkdirAll(repoPath, 0755)
	if err != nil {
		return err
	}
	if _, err := runCmd(repoPath, "git", "init"); err != nil {
		return err
	}
	if _, err := runCmd(repoPath, "git", "remote", "add", "origin", repo); err != nil {
		return err
	}
	return nil
}

// SetCredentials sets a local credentials file to connect to a remote git repository
func (m *NativeGitClient) SetCredentials(repo string, username string, password string, sshPrivateKey string, repoPath string) error {
	if password != "" {
		log.Infof("Setting password credentials")
		gitCredentialsFile := path.Join(repoPath, ".git", "credentials")
		repoURL, err := url.ParseRequestURI(repo)
		if err != nil {
			return err
		}
		repoURL.User = url.UserPassword(username, password)
		cmdURL := repoURL.String()
		err = ioutil.WriteFile(gitCredentialsFile, []byte(cmdURL), 0600)
		if err != nil {
			return fmt.Errorf("failed to set git credentials: %v", err)
		}
		_, err = runCmd(repoPath, "git", "config", "--local", "credential.helper", fmt.Sprintf("store --file=%s", gitCredentialsFile))
		if err != nil {
			return err
		}
	}
	if sshPrivateKey != "" {
		log.Infof("Setting SSH credentials")
		sshPrivateKeyFile := path.Join(repoPath, ".git", "ssh-private-key")
		err := ioutil.WriteFile(sshPrivateKeyFile, []byte(sshPrivateKey), 0600)
		if err != nil {
			return fmt.Errorf("failed to set git credentials: %v", err)
		}
		sshCmd := fmt.Sprintf("ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i %s", sshPrivateKeyFile)
		_, err = runCmd(repoPath, "git", "config", "--local", "core.sshCommand", sshCmd)
		if err != nil {
			return err
		}
	}
	return nil
}

// CloneOrFetch either clone or fetch repository into specified directory path.
func (m *NativeGitClient) CloneOrFetch(repo string, username string, password string, sshPrivateKey string, repoPath string) error {
	log.Debugf("Cloning/Fetching repo %s at %s", repo, repoPath)
	var needClone bool
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		needClone = true
	} else {
		cmd := exec.Command("git", "status")
		cmd.Dir = repoPath
		_, err = cmd.Output()
		needClone = err != nil
	}
	if needClone {
		_, err := exec.Command("rm", "-rf", repoPath).Output()
		if err != nil {
			return fmt.Errorf("unable to clean repo cache at %s: %v", repoPath, err)
		}
		err = m.Init(repo, repoPath)
		if err != nil {
			return fmt.Errorf("unable to clone repository %s: %v", repo, err)
		}
	}

	err := m.SetCredentials(repo, username, password, sshPrivateKey, repoPath)
	if err != nil {
		return err
	}
	// Fetch remote changes
	if _, err = runCmd(repoPath, "git", "fetch", "origin"); err != nil {
		return err
	}
	// git fetch does not update the HEAD reference. The following command will update the local
	// knowledge of what remote considers the “default branch”
	// See: https://stackoverflow.com/questions/8839958/how-does-origin-head-get-set
	if _, err := runCmd(repoPath, "git", "remote", "set-head", "origin", "-a"); err != nil {
		return err
	}
	// Delete all local branches (we must first detach so we are not checked out a branch we are about to delete)
	if _, err = runCmd(repoPath, "git", "checkout", "--detach", "origin/HEAD"); err != nil {
		return err
	}
	branchesOut, err := runCmd(repoPath, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return err
	}
	branchesOut = strings.TrimSpace(branchesOut)
	if branchesOut != "" {
		branches := strings.Split(branchesOut, "\n")
		args := []string{"branch", "-D"}
		args = append(args, branches...)
		if _, err = runCmd(repoPath, "git", args...); err != nil {
			return err
		}
	}
	return nil
}

// Reset resets local changes in a repository
func (m *NativeGitClient) Reset(repoPath string) error {
	if _, err := runCmd(repoPath, "git", "reset", "--hard", "origin/HEAD"); err != nil {
		return err
	}
	if _, err := runCmd(repoPath, "git", "clean", "-f"); err != nil {
		return err
	}
	return nil
}

// Checkout checkout specified git sha
func (m *NativeGitClient) Checkout(repoPath string, revision string) (string, error) {
	if revision == "" || revision == "HEAD" {
		revision = "origin/HEAD"
	}
	if _, err := runCmd(repoPath, "git", "checkout", revision); err != nil {
		return "", err
	}
	return m.CommitSHA(repoPath)
}

// CommitSHA returns current commit sha from `git rev-parse HEAD`
func (m *NativeGitClient) CommitSHA(repoPath string) (string, error) {
	out, err := runCmd(repoPath, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// NewNativeGitClient creates new instance of NativeGitClient
func NewNativeGitClient() (Client, error) {
	return &NativeGitClient{}, nil
}

// runCmd is a convenience function to run a command in a given directory and return its output
func runCmd(cwd string, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	log.Debug(strings.Join(cmd.Args, " "))
	cmd.Dir = cwd
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
