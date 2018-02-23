package git

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
)

// Client is a generic git client interface
type Client interface {
	CloneOrFetch(url string, username string, password string, repoPath string) error
	Checkout(repoPath string, sha string) error
}

// NativeGitClient implements Client interface using git CLI
type NativeGitClient struct {
	rootDirectoryPath string
}

// CloneOrFetch either clone or fetch repository into specified directory path.
func (m *NativeGitClient) CloneOrFetch(repo string, username string, password string, repoPath string) error {
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
		repoURL, err := url.ParseRequestURI(repo)
		if err != nil {
			return err
		}
		repoURL.User = url.UserPassword(username, password)
		_, err = exec.Command("git", "clone", repoURL.String(), repoPath).Output()
		if err != nil {
			return fmt.Errorf("unable to clone repository %s: %v", repoURL.String(), err)
		}
	} else {
		cmd := exec.Command("git", "fetch")
		cmd.Dir = repoPath
		_, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("unable to fetch repo %s: %v", repoPath, err)
		}
	}
	return nil
}

// Checkout checkout specified git sha
func (m *NativeGitClient) Checkout(repoPath string, sha string) error {
	if sha == "" {
		sha = "HEAD"
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
