package depot

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLsRemote(t *testing.T) {
	clnt, err := NewFactory().NewClient("https://github.com/argoproj/argo-cd.git", "git", "/tmp", "", "", "")
	assert.NoError(t, err)
	xpass := []string{
		"HEAD",
		"master",
		"release-0.8",
		"v0.8.0",
		"4e22a3cb21fa447ca362a05a505a69397c8a0d44",
		//"4e22a3c",
	}
	for _, revision := range xpass {
		commitSHA, err := clnt.LsRemote(revision)
		assert.NoError(t, err)
		assert.True(t, IsCommitSHA(commitSHA))
	}

	// We do not resolve truncated git hashes and return the commit as-is if it appears to be a commit
	commitSHA, err := clnt.LsRemote("4e22a3c")
	assert.NoError(t, err)
	assert.False(t, IsCommitSHA(commitSHA))
	assert.True(t, IsTruncatedCommitSHA(commitSHA))

	xfail := []string{
		"unresolvable",
		"4e22a3", // too short (6 characters)
	}
	for _, revision := range xfail {
		_, err := clnt.LsRemote(revision)
		assert.Error(t, err)
	}
}

func TestGitClient(t *testing.T) {
	testRepos := []string{
		"https://github.com/argoproj/argocd-example-apps",
		// TODO: add this back when azure repos are supported
		//"https://jsuen0437@dev.azure.com/jsuen0437/jsuen/_git/jsuen",
	}
	for _, repo := range testRepos {
		dirName, err := ioutil.TempDir("", "git-client-test-")
		assert.NoError(t, err)
		defer func() { _ = os.RemoveAll(dirName) }()

		clnt, err := NewFactory().NewClient(repo, "git", dirName, "", "", "")
		assert.NoError(t, err)

		testGitClient(t, clnt)
	}
}

// TestPrivateGitRepo tests the ability to operate on a private git repo. This test needs to be run
// manually since we do not have a private git repo for testing
//
// export TEST_REPO=https://github.com/jessesuen/private-argocd-example-apps
// export GITHUB_TOKEN=<YOURGITHUBTOKEN>
// go test -v -run ^(TestPrivateGitRepo)$ ./util/git/...
func TestPrivateGitRepo(t *testing.T) {
	repo := os.Getenv("TEST_REPO")
	username := os.Getenv("TEST_USERNAME")
	password := os.Getenv("GITHUB_TOKEN")
	if username == "" {
		username = "git" // username does not matter for tokens
	}
	if repo == "" || password == "" {
		t.Skip("skipping private git repo test since no repo or password supplied")
	}

	dirName, err := ioutil.TempDir("", "git-client-test-")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(dirName) }()

	clnt, err := NewFactory().NewClient(repo, "git", dirName, username, password, "")
	assert.NoError(t, err)

	testGitClient(t, clnt)
}

func testGitClient(t *testing.T, clnt Client) {
	commitSHA, err := clnt.LsRemote("HEAD")
	assert.NoError(t, err)

	err = clnt.Init()
	assert.NoError(t, err)

	err = clnt.Fetch()
	assert.NoError(t, err)

	// Do a second fetch to make sure we can treat `already up-to-date` error as not an error
	err = clnt.Fetch()
	assert.NoError(t, err)

	err = clnt.Checkout(".", commitSHA)
	assert.NoError(t, err)

	commitSHA2, err := clnt.CommitSHA(commitSHA)
	assert.NoError(t, err)

	assert.Equal(t, commitSHA, commitSHA2)
}
