package git

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/test/fixture/test_repos"
)

func TestIsCommitSHA(t *testing.T) {
	assert.True(t, IsCommitSHA("9d921f65f3c5373b682e2eb4b37afba6592e8f8b"))
	assert.True(t, IsCommitSHA("9D921F65F3C5373B682E2EB4B37AFBA6592E8F8B"))
	assert.False(t, IsCommitSHA("gd921f65f3c5373b682e2eb4b37afba6592e8f8b"))
	assert.False(t, IsCommitSHA("master"))
	assert.False(t, IsCommitSHA("HEAD"))
	assert.False(t, IsCommitSHA("9d921f6")) // only consider 40 characters hex strings as a commit-sha
	assert.True(t, IsTruncatedCommitSHA("9d921f6"))
	assert.False(t, IsTruncatedCommitSHA("9d921f")) // we only consider 7+ characters
	assert.False(t, IsTruncatedCommitSHA("branch-name"))
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
		isSSH, _ := IsSSHURL(k)
		assert.Equal(t, v, isSSH)
	}
}

func TestIsSSHURLUserName(t *testing.T) {
	isSSH, user := IsSSHURL("ssh://john@john-server.org:29418/project")
	assert.True(t, isSSH)
	assert.Equal(t, "john", user)

	isSSH, user = IsSSHURL("john@john-server.org:29418/project")
	assert.True(t, isSSH)
	assert.Equal(t, "john", user)
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
		assert.True(t, SameURL(k, v))
	}
}

func TestLsRemote(t *testing.T) {
	clnt, err := NewFactory().NewClient("https://github.com/argoproj/argo-cd.git", "/tmp", "", "", "", false)
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

func TestNewFactory(t *testing.T) {
	hackPath := test_repos.NewHackPath()
	defer func() { _ = hackPath.Close() }()

	type args struct {
		url, username, password, sshPrivateKey string
		insecureIgnoreHostKey                  bool
	}
	tests := []struct {
		name string
		args args
	}{
		{"Github", args{url: "https://github.com/argoproj/argocd-example-apps"}},
		{"Azure", args{url: "https://jsuen0437@dev.azure.com/jsuen0437/jsuen/_git/jsuen"}},
		{"PrivateRepo", args{test_repos.HTTPSTestRepo.URL, test_repos.HTTPSTestRepo.Username, test_repos.HTTPSTestRepo.Password, "", false}},
		{"PrivateSSHRepo", args{test_repos.SSHTestRepo.URL, "", "", test_repos.SSHTestRepo.SSHPrivateKey, false}},
	}
	for _, tt := range tests {
		dirName, err := ioutil.TempDir("", "git-client-test-")
		assert.NoError(t, err)
		defer func() { _ = os.RemoveAll(dirName) }()

		client, err := NewFactory().NewClient(tt.args.url, dirName, tt.args.username, tt.args.password, tt.args.sshPrivateKey, tt.args.insecureIgnoreHostKey)
		assert.NoError(t, err)
		commitSHA, err := client.LsRemote("HEAD")
		assert.NoError(t, err)

		err = client.Init()
		assert.NoError(t, err)

		err = client.Fetch()
		assert.NoError(t, err)

		// Do a second fetch to make sure we can treat `already up-to-date` error as not an error
		err = client.Fetch()
		assert.NoError(t, err)

		err = client.Checkout(commitSHA)
		assert.NoError(t, err)

		commitSHA2, err := client.CommitSHA()
		assert.NoError(t, err)

		assert.Equal(t, commitSHA, commitSHA2)
	}
}
