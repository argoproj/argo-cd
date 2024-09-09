package git

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/test/fixture/log"
	"github.com/argoproj/argo-cd/v2/test/fixture/path"
	"github.com/argoproj/argo-cd/v2/test/fixture/test"
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
	require.NoError(t, err)
	assert.NotEqual(t, "", certFile)

	keyFile, err := filepath.Abs("../../test/fixture/certs/argocd-test-client.key")
	require.NoError(t, err)
	assert.NotEqual(t, "", keyFile)

	certData, err := os.ReadFile(certFile)
	require.NoError(t, err)
	assert.NotEqual(t, "", string(certData))

	keyData, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	assert.NotEqual(t, "", string(keyData))

	// Get HTTPSCreds with client cert creds specified, and insecure connection
	creds := NewHTTPSCreds("test", "test", string(certData), string(keyData), false, "http://proxy:5000", "", &NoopCredsStore{}, false)
	client := GetRepoHTTPClient("https://localhost:9443/foo/bar", false, creds, "http://proxy:5000", "")
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
	if client.Transport != nil {
		transport := client.Transport.(*http.Transport)
		assert.NotNil(t, transport.TLSClientConfig)
		assert.True(t, transport.DisableKeepAlives)
		assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
		assert.NotNil(t, transport.TLSClientConfig.GetClientCertificate)
		assert.Nil(t, transport.TLSClientConfig.RootCAs)
		if transport.TLSClientConfig.GetClientCertificate != nil {
			cert, err := transport.TLSClientConfig.GetClientCertificate(nil)
			require.NoError(t, err)
			if err == nil {
				assert.NotNil(t, cert)
				assert.NotEmpty(t, cert.Certificate)
				assert.NotNil(t, cert.PrivateKey)
			}
		}
		proxy, err := transport.Proxy(nil)
		require.NoError(t, err)
		assert.NotNil(t, proxy) // nil would mean no proxy is used
		assert.Equal(t, "http://proxy:5000", proxy.String())
	}

	t.Setenv("http_proxy", "http://proxy-from-env:7878")

	// Get HTTPSCreds without client cert creds, but insecure connection
	creds = NewHTTPSCreds("test", "test", "", "", true, "", "", &NoopCredsStore{}, false)
	client = GetRepoHTTPClient("https://localhost:9443/foo/bar", true, creds, "", "")
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
	if client.Transport != nil {
		transport := client.Transport.(*http.Transport)
		assert.NotNil(t, transport.TLSClientConfig)
		assert.True(t, transport.DisableKeepAlives)
		assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
		assert.NotNil(t, transport.TLSClientConfig.GetClientCertificate)
		assert.Nil(t, transport.TLSClientConfig.RootCAs)
		if transport.TLSClientConfig.GetClientCertificate != nil {
			cert, err := transport.TLSClientConfig.GetClientCertificate(nil)
			require.NoError(t, err)
			if err == nil {
				assert.NotNil(t, cert)
				assert.Empty(t, cert.Certificate)
				assert.Nil(t, cert.PrivateKey)
			}
		}
		req, err := http.NewRequest(http.MethodGet, "http://proxy-from-env:7878", nil)
		require.NoError(t, err)
		proxy, err := transport.Proxy(req)
		require.NoError(t, err)
		assert.Equal(t, "http://proxy-from-env:7878", proxy.String())
	}
	// GetRepoHTTPClient with root ca
	cert, err := os.ReadFile("../../test/fixture/certs/argocd-test-server.crt")
	require.NoError(t, err)
	temppath := t.TempDir()
	defer os.RemoveAll(temppath)
	err = os.WriteFile(filepath.Join(temppath, "127.0.0.1"), cert, 0o666)
	require.NoError(t, err)
	t.Setenv(common.EnvVarTLSDataPath, temppath)
	client = GetRepoHTTPClient("https://127.0.0.1", false, creds, "", "")
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)
	if client.Transport != nil {
		transport := client.Transport.(*http.Transport)
		assert.NotNil(t, transport.TLSClientConfig)
		assert.True(t, transport.DisableKeepAlives)
		assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
		assert.NotNil(t, transport.TLSClientConfig.RootCAs)
	}
}

func TestLsRemote(t *testing.T) {
	clnt, err := NewClientExt("https://github.com/argoproj/argo-cd.git", "/tmp", NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	testCases := []struct {
		name           string
		revision       string
		expectedCommit string
	}{
		{
			name:     "should resolve symbolic link reference",
			revision: "HEAD",
		},
		{
			name:     "should resolve branch name",
			revision: "master",
		},
		{
			name:           "should resolve tag without semantic versioning",
			revision:       "release-0.8",
			expectedCommit: "ff87d8cb9e669d3738434733ecba3c6dd2c64d70",
		},
		{
			name:           "should resolve a pined tag with semantic versioning",
			revision:       "v0.8.0",
			expectedCommit: "d7c04ae24c16f8ec611b0331596fbc595537abe9",
		},
		{
			name:           "should resolve a pined tag with semantic versioning without the 'v' prefix",
			revision:       "0.8.0",
			expectedCommit: "d7c04ae24c16f8ec611b0331596fbc595537abe9",
		},
		{
			name:           "should resolve a range tag with semantic versioning",
			revision:       "v0.8.*", // it should resolve to v0.8.2
			expectedCommit: "e5eefa2b943ae14a3e4491d4e35ef082e1c2a3f4",
		},
		{
			name:           "should resolve a range tag with semantic versioning without the 'v' prefix",
			revision:       "0.8.*", // it should resolve to v0.8.2
			expectedCommit: "e5eefa2b943ae14a3e4491d4e35ef082e1c2a3f4",
		},
		{
			name:           "should resolve a conditional range tag with semantic versioning",
			revision:       ">=v2.9.0 <2.10.4", // it should resolve to v2.10.3
			expectedCommit: "0fd6344537eb948cff602824a1d060421ceff40e",
		},
		{
			name:     "should resolve a star range tag with semantic versioning",
			revision: "*",
		},
		{
			name:     "should resolve a star range suffixed tag with semantic versioning",
			revision: "*-0",
		},
		{
			name:           "should resolve commit sha",
			revision:       "4e22a3cb21fa447ca362a05a505a69397c8a0d44",
			expectedCommit: "4e22a3cb21fa447ca362a05a505a69397c8a0d44",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			commitSHA, err := clnt.LsRemote(tc.revision)
			require.NoError(t, err)
			assert.True(t, IsCommitSHA(commitSHA))
			if tc.expectedCommit != "" {
				assert.Equal(t, tc.expectedCommit, commitSHA)
			}
		})
	}

	// We do not resolve truncated git hashes and return the commit as-is if it appears to be a commit
	t.Run("truncated commit", func(t *testing.T) {
		commitSHA, err := clnt.LsRemote("4e22a3c")
		require.NoError(t, err)
		assert.False(t, IsCommitSHA(commitSHA))
		assert.True(t, IsTruncatedCommitSHA(commitSHA))
	})

	t.Run("unresolvable revisions", func(t *testing.T) {
		xfail := []string{
			"unresolvable",
			"4e22a3", // too short (6 characters)
		}

		for _, revision := range xfail {
			_, err := clnt.LsRemote(revision)
			assert.ErrorContains(t, err, "Unable to resolve")
		}
	})
}

// Running this test requires git-lfs to be installed on your machine.
func TestLFSClient(t *testing.T) {
	// temporary disable LFS test
	// TODO(alexmt): dockerize tests in and enabled it
	t.Skip()

	tempDir := t.TempDir()

	client, err := NewClientExt("https://github.com/argoproj-labs/argocd-testrepo-lfs", tempDir, NopCreds{}, false, true, "", "")
	require.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	require.NoError(t, err)
	assert.NotEqual(t, "", commitSHA)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)

	err = client.Checkout(commitSHA, true)
	require.NoError(t, err)

	largeFiles, err := client.LsLargeFiles()
	require.NoError(t, err)
	assert.Len(t, largeFiles, 3)

	fileHandle, err := os.Open(fmt.Sprintf("%s/test3.yaml", tempDir))
	require.NoError(t, err)
	if err == nil {
		defer func() {
			if err = fileHandle.Close(); err != nil {
				require.NoError(t, err)
			}
		}()
		text, err := io.ReadAll(fileHandle)
		require.NoError(t, err)
		if err == nil {
			assert.Equal(t, "This is not a YAML, sorry.\n", string(text))
		}
	}
}

func TestVerifyCommitSignature(t *testing.T) {
	p := t.TempDir()

	client, err := NewClientExt("https://github.com/argoproj/argo-cd.git", p, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("")
	require.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	require.NoError(t, err)

	err = client.Checkout(commitSHA, true)
	require.NoError(t, err)

	// 28027897aad1262662096745f2ce2d4c74d02b7f is a commit that is signed in the repo
	// It doesn't matter whether we know the key or not at this stage
	{
		out, err := client.VerifyCommitSignature("28027897aad1262662096745f2ce2d4c74d02b7f")
		require.NoError(t, err)
		assert.NotEmpty(t, out)
		assert.Contains(t, out, "gpg: Signature made")
	}

	// 85d660f0b967960becce3d49bd51c678ba2a5d24 is a commit that is not signed
	{
		out, err := client.VerifyCommitSignature("85d660f0b967960becce3d49bd51c678ba2a5d24")
		require.NoError(t, err)
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
		{"GitHub", args{url: "https://github.com/argoproj/argocd-example-apps"}},
	}
	for _, tt := range tests {
		if tt.name == "PrivateSSHRepo" {
			test.Flaky(t)
		}

		dirName := t.TempDir()

		client, err := NewClientExt(tt.args.url, dirName, NopCreds{}, tt.args.insecureIgnoreHostKey, false, "", "")
		require.NoError(t, err)
		commitSHA, err := client.LsRemote("HEAD")
		require.NoError(t, err)

		err = client.Init()
		require.NoError(t, err)

		err = client.Fetch("")
		require.NoError(t, err)

		// Do a second fetch to make sure we can treat `already up-to-date` error as not an error
		err = client.Fetch("")
		require.NoError(t, err)

		err = client.Checkout(commitSHA, true)
		require.NoError(t, err)

		revisionMetadata, err := client.RevisionMetadata(commitSHA)
		require.NoError(t, err)
		assert.NotNil(t, revisionMetadata)
		assert.Regexp(t, "^.*<.*>$", revisionMetadata.Author)
		assert.Empty(t, revisionMetadata.Tags)
		assert.NotEmpty(t, revisionMetadata.Date)
		assert.NotEmpty(t, revisionMetadata.Message)

		commitSHA2, err := client.CommitSHA()
		require.NoError(t, err)

		assert.Equal(t, commitSHA, commitSHA2)
	}
}

func TestListRevisions(t *testing.T) {
	dir := t.TempDir()

	repoURL := "https://github.com/argoproj/argo-cd.git"
	client, err := NewClientExt(repoURL, dir, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	lsResult, err := client.LsRefs()
	require.NoError(t, err)

	testBranch := "master"
	testTag := "v1.0.0"

	assert.Contains(t, lsResult.Branches, testBranch)
	assert.Contains(t, lsResult.Tags, testTag)
	assert.NotContains(t, lsResult.Branches, testTag)
	assert.NotContains(t, lsResult.Tags, testBranch)
}

func TestLsFiles(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	client, err := NewClientExt("", tmpDir1, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	err = runCmd(tmpDir1, "git", "init")
	require.NoError(t, err)

	// Prepare files
	a, err := os.Create(filepath.Join(tmpDir1, "a.yaml"))
	require.NoError(t, err)
	a.Close()
	err = os.MkdirAll(filepath.Join(tmpDir1, "subdir"), 0o755)
	require.NoError(t, err)
	b, err := os.Create(filepath.Join(tmpDir1, "subdir", "b.yaml"))
	require.NoError(t, err)
	b.Close()
	err = os.MkdirAll(filepath.Join(tmpDir2, "subdir"), 0o755)
	require.NoError(t, err)
	c, err := os.Create(filepath.Join(tmpDir2, "c.yaml"))
	require.NoError(t, err)
	c.Close()
	err = os.Symlink(filepath.Join(tmpDir2, "c.yaml"), filepath.Join(tmpDir1, "link.yaml"))
	require.NoError(t, err)

	err = runCmd(tmpDir1, "git", "add", ".")
	require.NoError(t, err)
	err = runCmd(tmpDir1, "git", "commit", "-m", "Initial commit")
	require.NoError(t, err)

	// Old and default globbing
	expectedResult := []string{"a.yaml", "link.yaml", "subdir/b.yaml"}
	lsResult, err := client.LsFiles("*.yaml", false)
	require.NoError(t, err)
	assert.Equal(t, expectedResult, lsResult)

	// New and safer globbing, do not return symlinks resolving outside of the repo
	expectedResult = []string{"a.yaml"}
	lsResult, err = client.LsFiles("*.yaml", true)
	require.NoError(t, err)
	assert.Equal(t, expectedResult, lsResult)

	// New globbing, do not return files outside of the repo
	var nilResult []string
	lsResult, err = client.LsFiles(filepath.Join(tmpDir2, "*.yaml"), true)
	require.NoError(t, err)
	assert.Equal(t, nilResult, lsResult)
}
