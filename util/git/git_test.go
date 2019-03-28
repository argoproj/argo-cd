package git

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO: move this into shared test package after resolving import cycle
const (
	// This is a throwaway gitlab test account/repo with a read-only personal access token for the
	// purposes of testing private git repos
	PrivateGitRepo     = "https://gitlab.com/argo-cd-test/test-apps.git"
	PrivateGitUsername = "blah"
	PrivateGitPassword = "B5sBDeoqAVUouoHkrovy"
)

func TestIsCommitSHA(t *testing.T) {
	assert.True(t, isCommitSHA("9d921f65f3c5373b682e2eb4b37afba6592e8f8b"))
	assert.True(t, isCommitSHA("9D921F65F3C5373B682E2EB4B37AFBA6592E8F8B"))
	assert.False(t, isCommitSHA("gd921f65f3c5373b682e2eb4b37afba6592e8f8b"))
	assert.False(t, isCommitSHA("master"))
	assert.False(t, isCommitSHA("HEAD"))
	assert.False(t, isCommitSHA("9d921f6")) // only consider 40 characters hex strings as a commit-sha
	assert.True(t, isTruncatedCommitSHA("9d921f6"))
	assert.False(t, isTruncatedCommitSHA("9d921f")) // we only consider 7+ characters
	assert.False(t, isTruncatedCommitSHA("branch-name"))
}

func TestEnsurePrefix(t *testing.T) {
	data := [][]string{
		{"world", "hello", "helloworld"},
		{"helloworld", "hello", "helloworld"},
		{"example.com", "https://", "https://example.com"},
		{"https://example.com", "https://", "https://example.com"},
		{"cd", "argo", "argocd"},
		{"argocd", "argo", "argocd"},
		{"", "argocd", "argocd"},
		{"argocd", "", "argocd"},
	}
	for _, table := range data {
		result := ensurePrefix(table[0], table[1])
		assert.Equal(t, table[2], result)
	}
}

func TestRemoveSuffix(t *testing.T) {
	data := [][]string{
		{"hello.git", ".git", "hello"},
		{"hello", ".git", "hello"},
		{".git", ".git", ""},
	}
	for _, table := range data {
		result := removeSuffix(table[0], table[1])
		assert.Equal(t, table[2], result)
	}
}

func TestIsSSHURL(t *testing.T) {
	data := map[string]bool{
		"git://github.com/argoproj/test.git":     false,
		"git@GITHUB.com:argoproj/test.git":       true,
		"git@github.com:test":                    true,
		"git@github.com:test.git":                true,
		"https://github.com/argoproj/test":       false,
		"https://github.com/argoproj/test.git":   false,
		"ssh://git@GITHUB.com:argoproj/test":     true,
		"ssh://git@GITHUB.com:argoproj/test.git": true,
		"ssh://git@github.com:test.git":          true,
	}
	for k, v := range data {
		assert.Equal(t, v, isSSHURL(k))
	}
}

func TestSameURL(t *testing.T) {
	data := map[string]string{
		"git@GITHUB.com:argoproj/test":                     "git@github.com:argoproj/test.git",
		"git@GITHUB.com:argoproj/test.git":                 "git@github.com:argoproj/test.git",
		"git@GITHUB.com:test":                              "git@github.com:test.git",
		"git@GITHUB.com:test.git":                          "git@github.com:test.git",
		"https://GITHUB.com/argoproj/test":                 "https://github.com/argoproj/test.git",
		"https://GITHUB.com/argoproj/test.git":             "https://github.com/argoproj/test.git",
		"https://github.com/FOO":                           "https://github.com/foo",
		"https://github.com/TEST":                          "https://github.com/TEST.git",
		"https://github.com/TEST.git":                      "https://github.com/TEST.git",
		"ssh://git@GITHUB.com:argoproj/test":               "git@github.com:argoproj/test.git",
		"ssh://git@GITHUB.com:argoproj/test.git":           "git@github.com:argoproj/test.git",
		"ssh://git@GITHUB.com:test.git":                    "git@github.com:test.git",
		"ssh://git@github.com:test":                        "git@github.com:test.git",
		" https://github.com/argoproj/test ":               "https://github.com/argoproj/test.git",
		"\thttps://github.com/argoproj/test\n":             "https://github.com/argoproj/test.git",
		"https://1234.visualstudio.com/myproj/_git/myrepo": "https://1234.visualstudio.com/myproj/_git/myrepo",
		"https://dev.azure.com/1234/myproj/_git/myrepo":    "https://dev.azure.com/1234/myproj/_git/myrepo",
	}
	for k, v := range data {
		assert.True(t, sameURL(k, v))
	}
}

func TestLsRemote(t *testing.T) {
	clnt, err := newFactory().newClient("https://github.com/argoproj/argo-cd.git", "/tmp", "", "", "", false)
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
		commitSHA, err := clnt.lsRemote(revision)
		assert.NoError(t, err)
		assert.True(t, isCommitSHA(commitSHA))
	}

	// We do not resolve truncated git hashes and return the commit as-is if it appears to be a commit
	commitSHA, err := clnt.lsRemote("4e22a3c")
	assert.NoError(t, err)
	assert.False(t, isCommitSHA(commitSHA))
	assert.True(t, isTruncatedCommitSHA(commitSHA))

	xfail := []string{
		"unresolvable",
		"4e22a3", // too short (6 characters)
	}
	for _, revision := range xfail {
		_, err := clnt.lsRemote(revision)
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

		clnt, err := newFactory().newClient(repo, dirName, "", "", "", false)
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

	clnt, err := newFactory().newClient(PrivateGitRepo, dirName, PrivateGitUsername, PrivateGitPassword, "", false)
	assert.NoError(t, err)

	testGitClient(t, clnt)
}

func testGitClient(t *testing.T, clnt client) {
	commitSHA, err := clnt.lsRemote("HEAD")
	assert.NoError(t, err)

	err = clnt.init()
	assert.NoError(t, err)

	err = clnt.fetch()
	assert.NoError(t, err)

	// Do a second fetch to make sure we can treat `already up-to-date` error as not an error
	err = clnt.fetch()
	assert.NoError(t, err)

	err = clnt.checkout(commitSHA)
	assert.NoError(t, err)

	commitSHA2, err := clnt.commitSHA()
	assert.NoError(t, err)

	assert.Equal(t, commitSHA, commitSHA2)
}
