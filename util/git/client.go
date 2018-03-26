package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// Client is a generic git client interface
type Client interface {
	CloneOrFetch(url string, username string, password string, sshPrivateKey string, repoPath string) error
	Checkout(repoPath string, sha string) error
	Reset(repoPath string) error
}

// NativeGitClient implements Client interface using git CLI
type NativeGitClient struct {
	rootDirectoryPath string
}

// CloneOrFetch either clone or fetch repository into specified directory path.
func (m *NativeGitClient) CloneOrFetch(repo string, username string, password string, sshPrivateKey string, repoPath string) error {
	var needClone bool
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		needClone = true
	} else {
		cmd := exec.Command("git", "status")
		cmd.Dir = repoPath
		_, err = cmd.Output()
		needClone = err != nil
	}

	repoURL, env, err := GetGitCommandEnvAndURL(repo, username, password, sshPrivateKey)
	if err != nil {
		return err
	}

	if needClone {
		_, err := exec.Command("rm", "-rf", repoPath).Output()
		if err != nil {
			return fmt.Errorf("unable to clean repo cache at %s: %v", repoPath, err)
		}

		log.Infof("Cloning %s to %s", repo, repoPath)
		cmd := exec.Command("git", "clone", repoURL, repoPath)
		cmd.Env = env
		_, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("unable to clone repository %s: %v", repo, err)
		}
	} else {
		log.Infof("Fetching %s", repo)
		// Fetch remote changes and delete all local branches
		cmd := exec.Command("sh", "-c", "git fetch --all && git checkout --detach HEAD")
		cmd.Env = env
		cmd.Dir = repoPath
		_, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("unable to fetch repo %s: %v", repoPath, err)
		}

		cmd = exec.Command("sh", "-c", "for i in $(git branch --merged | grep -v \\*); do git branch -D $i; done")
		cmd.Dir = repoPath
		_, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("unable to delete local branches for %s: %v", repoPath, err)
		}

	}
	return nil
}

// Reset resets local changes
func (m *NativeGitClient) Reset(repoPath string) error {
	cmd := exec.Command("sh", "-c", "git reset --hard HEAD && git clean -f")
	cmd.Dir = repoPath
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("unable to reset repository %s: %v", repoPath, err)
	}

	return nil
}

// Checkout checkout specified git sha
func (m *NativeGitClient) Checkout(repoPath string, sha string) error {
	if sha == "" {
		sha = "origin/HEAD"
	}
	cmd := exec.Command("git", "checkout", sha)
	cmd.Dir = repoPath
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("unable to checkout revision %s: %v", sha, err)
	}

	return nil

}

// NewNativeGitClient creates new instance of NativeGitClient
func NewNativeGitClient() (Client, error) {
	rootDirPath, err := ioutil.TempDir("", "argo-git")
	if err != nil {
		return nil, err
	}
	return &NativeGitClient{
		rootDirectoryPath: rootDirPath,
	}, nil
}
