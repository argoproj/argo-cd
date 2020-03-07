package git

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/test/fixture/log"
	"github.com/argoproj/argo-cd/test/fixture/path"
	"github.com/argoproj/argo-cd/test/fixture/test"
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

	isSSH, user = IsSSHURL("john@doe.org@john-server.org:29418/project")
	assert.True(t, isSSH)
	assert.Equal(t, "john@doe.org", user)

	isSSH, user = IsSSHURL("ssh://john@doe.org@john-server.org:29418/project")
	assert.True(t, isSSH)
	assert.Equal(t, "john@doe.org", user)

	isSSH, user = IsSSHURL("john@doe.org@john-server.org:project")
	assert.True(t, isSSH)
	assert.Equal(t, "john@doe.org", user)

	isSSH, user = IsSSHURL("john@doe.org@john-server.org:29418/project")
	assert.True(t, isSSH)
	assert.Equal(t, "john@doe.org", user)

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
		"https://github.com:4443/TEST":                     "https://github.com:4443/TEST.git",
		"https://github.com:4443/TEST.git":                 "https://github.com:4443/TEST",
		"ssh://git@GITHUB.com/argoproj/test":               "git@github.com:argoproj/test.git",
		"ssh://git@GITHUB.com/argoproj/test.git":           "git@github.com:argoproj/test.git",
		"ssh://git@GITHUB.com/test.git":                    "git@github.com:test.git",
		"ssh://git@github.com/test":                        "git@github.com:test.git",
		" https://github.com/argoproj/test ":               "https://github.com/argoproj/test.git",
		"\thttps://github.com/argoproj/test\n":             "https://github.com/argoproj/test.git",
		"https://1234.visualstudio.com/myproj/_git/myrepo": "https://1234.visualstudio.com/myproj/_git/myrepo",
		"https://dev.azure.com/1234/myproj/_git/myrepo":    "https://dev.azure.com/1234/myproj/_git/myrepo",
	}
	for k, v := range data {
		assert.True(t, SameURL(k, v))
	}
}

func TestCustomHTTPClient(t *testing.T) {
	certFile, err := filepath.Abs("../../test/fixture/certs/argocd-test-client.crt")
	assert.NoError(t, err)
	assert.NotEqual(t, "", certFile)

	keyFile, err := filepath.Abs("../../test/fixture/certs/argocd-test-client.key")
	assert.NoError(t, err)
	assert.NotEqual(t, "", keyFile)

	certData, err := ioutil.ReadFile(certFile)
	assert.NoError(t, err)
	assert.NotEqual(t, "", string(certData))

	keyData, err := ioutil.ReadFile(keyFile)
	assert.NoError(t, err)
	assert.NotEqual(t, "", string(keyData))

	// Get HTTPSCreds with client cert creds specified, and insecure connection
	creds := NewHTTPSCreds("test", "test", string(certData), string(keyData), false)
	client := GetRepoHTTPClient("https://localhost:9443/foo/bar", false, creds)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
	if client.Transport != nil {
		httpClient := client.Transport.(*http.Transport)
		assert.NotNil(t, httpClient.TLSClientConfig)

		assert.Equal(t, false, httpClient.TLSClientConfig.InsecureSkipVerify)

		assert.NotNil(t, httpClient.TLSClientConfig.GetClientCertificate)
		if httpClient.TLSClientConfig.GetClientCertificate != nil {
			cert, err := httpClient.TLSClientConfig.GetClientCertificate(nil)
			assert.NoError(t, err)
			if err == nil {
				assert.NotNil(t, cert)
				assert.NotEqual(t, 0, len(cert.Certificate))
				assert.NotNil(t, cert.PrivateKey)
			}
		}
	}

	// Get HTTPSCreds without client cert creds, but insecure connection
	creds = NewHTTPSCreds("test", "test", "", "", true)
	client = GetRepoHTTPClient("https://localhost:9443/foo/bar", true, creds)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
	if client.Transport != nil {
		httpClient := client.Transport.(*http.Transport)
		assert.NotNil(t, httpClient.TLSClientConfig)

		assert.Equal(t, true, httpClient.TLSClientConfig.InsecureSkipVerify)

		assert.NotNil(t, httpClient.TLSClientConfig.GetClientCertificate)
		if httpClient.TLSClientConfig.GetClientCertificate != nil {
			cert, err := httpClient.TLSClientConfig.GetClientCertificate(nil)
			assert.NoError(t, err)
			if err == nil {
				assert.NotNil(t, cert)
				assert.Equal(t, 0, len(cert.Certificate))
				assert.Nil(t, cert.PrivateKey)
			}
		}
	}
}

func TestLsRemote(t *testing.T) {
	clnt, err := NewClientExt("https://github.com/argoproj/argo-cd.git", "/tmp", NopCreds{}, false, false)
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

// Running this test requires git-lfs to be installed on your machine.
func TestLFSClient(t *testing.T) {

	// temporary disable LFS test
	// TODO(alexmt): dockerize tests in and enabled it
	t.Skip()

	tempDir, err := ioutil.TempDir("", "git-client-lfs-test-")
	assert.NoError(t, err)
	if err == nil {
		defer func() { _ = os.RemoveAll(tempDir) }()
	}

	client, err := NewClientExt("https://github.com/argoproj-labs/argocd-testrepo-lfs", tempDir, NopCreds{}, false, true)
	assert.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	assert.NoError(t, err)
	assert.NotEqual(t, "", commitSHA)

	err = client.Init()
	assert.NoError(t, err)

	err = client.Fetch()
	assert.NoError(t, err)

	err = client.Checkout(commitSHA)
	assert.NoError(t, err)

	largeFiles, err := client.LsLargeFiles()
	assert.NoError(t, err)
	assert.Equal(t, 3, len(largeFiles))

	fileHandle, err := os.Open(fmt.Sprintf("%s/test3.yaml", tempDir))
	assert.NoError(t, err)
	if err == nil {
		defer fileHandle.Close()
		text, err := ioutil.ReadAll(fileHandle)
		assert.NoError(t, err)
		if err == nil {
			assert.Equal(t, "This is not a YAML, sorry.\n", string(text))
		}
	}
}

func TestVerifyCommitSignature(t *testing.T) {
	p, err := ioutil.TempDir("", "test-verify-commit-sig")
	if err != nil {
		panic(err.Error())
	}
	defer os.RemoveAll(p)

	client, err := NewClientExt("https://github.com/argoproj/argo-cd.git", p, NopCreds{}, false, false)
	assert.NoError(t, err)

	err = client.Init()
	assert.NoError(t, err)

	err = client.Fetch()
	assert.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	assert.NoError(t, err)

	err = client.Checkout(commitSHA)
	assert.NoError(t, err)

	// 28027897aad1262662096745f2ce2d4c74d02b7f is a commit that is signed in the repo
	// It doesn't matter whether we know the key or not at this stage
	{
		out, err := client.VerifyCommitSignature("28027897aad1262662096745f2ce2d4c74d02b7f")
		assert.NoError(t, err)
		assert.NotEmpty(t, out)
		assert.Contains(t, out, "gpg: Signature made")
	}

	// 85d660f0b967960becce3d49bd51c678ba2a5d24 is a commit that is not signed
	{
		out, err := client.VerifyCommitSignature("85d660f0b967960becce3d49bd51c678ba2a5d24")
		assert.NoError(t, err)
		assert.Empty(t, out)
	}
}

func TestNewFactory(t *testing.T) {
	addBinDirToPath := path.NewBinDirToPath()
	defer addBinDirToPath.Close()
	closer := log.Debug()
	defer closer()

	type args struct {
		url                   string
		insecureIgnoreHostKey bool
	}
	tests := []struct {
		name string
		args args
	}{
		{"Github", args{url: "https://github.com/argoproj/argocd-example-apps"}},
		{"Azure", args{url: "https://jsuen0437@dev.azure.com/jsuen0437/jsuen/_git/jsuen"}},
	}
	for _, tt := range tests {

		if tt.name == "PrivateSSHRepo" {
			test.Flaky(t)
		}

		dirName, err := ioutil.TempDir("", "git-client-test-")
		assert.NoError(t, err)
		defer func() { _ = os.RemoveAll(dirName) }()

		client, err := NewClientExt(tt.args.url, dirName, NopCreds{}, tt.args.insecureIgnoreHostKey, false)
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

		revisionMetadata, err := client.RevisionMetadata(commitSHA)
		assert.NoError(t, err)
		assert.NotNil(t, revisionMetadata)
		assert.Regexp(t, "^.*<.*>$", revisionMetadata.Author)
		assert.Len(t, revisionMetadata.Tags, 0)
		assert.NotEmpty(t, revisionMetadata.Date)
		assert.NotEmpty(t, revisionMetadata.Message)

		commitSHA2, err := client.CommitSHA()
		assert.NoError(t, err)

		assert.Equal(t, commitSHA, commitSHA2)
	}
}
