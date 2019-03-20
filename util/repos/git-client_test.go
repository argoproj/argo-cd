package repos

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveRevision(t *testing.T) {
	clnt, err := NewFactory().NewClient(Config{Url: "https://github.com/argoproj/argo-cd.git"}, "/tmp")
	assert.NoError(t, err)
	xpass := []string{
		"",
		"master",
		"release-0.8",
		"v0.8.0",
		"4e22a3cb21fa447ca362a05a505a69397c8a0d44",
		//"4e22a3c",
	}
	for _, revision := range xpass {
		commitSHA, err := clnt.ResolveRevision(revision)
		assert.NoError(t, err)
		assert.True(t, IsCommitSHA(commitSHA))
	}

	// We do not resolve truncated git hashes and return the commit as-is if it appears to be a commit
	commitSHA, err := clnt.ResolveRevision("4e22a3c")
	assert.NoError(t, err)
	assert.False(t, IsCommitSHA(commitSHA))
	assert.True(t, IsTruncatedCommitSHA(commitSHA))

	xfail := []string{
		"unresolvable",
		"4e22a3", // too short (6 characters)
	}
	for _, revision := range xfail {
		_, err := clnt.ResolveRevision(revision)
		assert.Error(t, err)
	}
}

func TestGitClient(t *testing.T) {
	testRepos := []string{
		"https://github.com/argoproj/argocd-example-apps",
		"https://jsuen0437@dev.azure.com/jsuen0437/jsuen/_git/jsuen",
	}
	for _, repo := range testRepos {
		dirName, err := ioutil.TempDir("", "git-client-test-")
		assert.NoError(t, err)
		defer func() { _ = os.RemoveAll(dirName) }()

		clnt, err := NewFactory().NewClient(Config{Url: repo}, dirName)
		assert.NoError(t, err)

		testGitClient(t, clnt)
	}
}

// TestPrivateGitRepo tests the ability to operate on a private git repo.
func TestPrivateGitRepo(t *testing.T) {
	// add the hack path which has the git-ask-pass.sh shell script
	osPath := os.Getenv("PATH")
	hackPath, err := filepath.Abs("../../hack")
	assert.NoError(t, err)
	err = os.Setenv("PATH", fmt.Sprintf("%s:%s", osPath, hackPath))
	assert.NoError(t, err)
	defer func() { _ = os.Setenv("PATH", osPath) }()

	dirName, err := ioutil.TempDir("", "git-client-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(dirName) }()

	clnt, err := NewFactory().NewClient(Config{Url: PrivateGitRepo, Username: PrivateGitUsername, Password: PrivateGitPassword}, dirName)
	assert.NoError(t, err)

	testGitClient(t, clnt)
}

func testGitClient(t *testing.T, clnt Client) {
	commitSHA, err := clnt.ResolveRevision("")
	assert.NoError(t, err)

	// Do a second fetch to make sure we can treat `already up-to-date` error as not an error
	_, err = clnt.Checkout(".", commitSHA)
	assert.NoError(t, err)

	commitSHA2, err := clnt.Checkout(".", commitSHA)
	assert.NoError(t, err)

	assert.Equal(t, commitSHA, commitSHA2)
}
