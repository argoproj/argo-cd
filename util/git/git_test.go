package git

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/test/fixture/log"
	"github.com/argoproj/argo-cd/v3/test/fixture/path"
	"github.com/argoproj/argo-cd/v3/test/fixture/test"
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
		" https://github.com/argoproj/test ":               "https://github.com/argoproj/test.git", //nolint:gocritic // This includes whitespaces for testing
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
	assert.NotEmpty(t, certFile)

	keyFile, err := filepath.Abs("../../test/fixture/certs/argocd-test-client.key")
	require.NoError(t, err)
	assert.NotEmpty(t, keyFile)

	certData, err := os.ReadFile(certFile)
	require.NoError(t, err)
	assert.NotEmpty(t, string(certData))

	keyData, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	assert.NotEmpty(t, string(keyData))

	// Get HTTPSCreds with client cert creds specified, and insecure connection
	creds := NewHTTPSCreds("test", "test", "", string(certData), string(keyData), false, &NoopCredsStore{}, false)
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
	creds = NewHTTPSCreds("test", "test", "", "", "", true, &NoopCredsStore{}, false)
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
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://proxy-from-env:7878", http.NoBody)
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
			name:           "should resolve a pinned tag with semantic versioning",
			revision:       "v0.8.0",
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
			assert.ErrorContains(t, err, "unable to resolve")
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
	assert.NotEmpty(t, commitSHA)

	err = client.Init()
	require.NoError(t, err)

	err = client.Fetch("", 0, false)
	require.NoError(t, err)

	_, err = client.Checkout(commitSHA, true)
	require.NoError(t, err)

	largeFiles, err := client.LsLargeFiles()
	require.NoError(t, err)
	assert.Len(t, largeFiles, 3)

	fileHandle, err := os.Open(tempDir + "/test3.yaml")
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

	err = client.Fetch("", 0, false)
	require.NoError(t, err)

	commitSHA, err := client.LsRemote("HEAD")
	require.NoError(t, err)

	_, err = client.Checkout(commitSHA, true)
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
	addBinDirToPath := path.NewBinDirToPath(t)
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

		err = client.Fetch("", 0, false)
		require.NoError(t, err)

		// Do a second fetch to make sure we can treat `already up-to-date` error as not an error
		err = client.Fetch("", 0, false)
		require.NoError(t, err)

		_, err = client.Checkout(commitSHA, true)
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
	ctx := t.Context()

	client, err := NewClientExt("", tmpDir1, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	require.NoError(t, runCmd(ctx, tmpDir1, "git", "init"))

	// Setup files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir1, "a.yaml"), []byte{}, 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir1, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir1, "subdir", "b.yaml"), []byte{}, 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir2, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir2, "c.yaml"), []byte{}, 0o644))

	require.NoError(t, os.Symlink(filepath.Join(tmpDir2, "c.yaml"), filepath.Join(tmpDir1, "link.yaml")))

	require.NoError(t, runCmd(ctx, tmpDir1, "git", "add", "."))
	require.NoError(t, runCmd(ctx, tmpDir1, "git", "commit", "-m", "Initial commit"))

	tests := []struct {
		name           string
		pattern        string
		safeGlobbing   bool
		expectedResult []string
	}{
		{
			name:           "Old globbing with symlinks and subdir",
			pattern:        "*.yaml",
			safeGlobbing:   false,
			expectedResult: []string{"a.yaml", "link.yaml", "subdir/b.yaml"},
		},
		{
			name:           "Safe globbing excludes symlinks",
			pattern:        "*.yaml",
			safeGlobbing:   true,
			expectedResult: []string{"a.yaml"},
		},
		{
			name:           "Safe globbing excludes external paths",
			pattern:        filepath.Join(tmpDir2, "*.yaml"),
			safeGlobbing:   true,
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lsResult, err := client.LsFiles(tt.pattern, tt.safeGlobbing)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, lsResult)
		})
	}
}

func TestLsFilesForGitFileGeneratorGlobbingPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := t.Context()

	client, err := NewClientExt("", tmpDir, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	err = runCmd(ctx, tmpDir, "git", "init")
	require.NoError(t, err)

	// Setup directory structure and files
	files := []string{
		"cluster-charts/cluster1/mychart/charts/mysubchart/values.yaml",
		"cluster-charts/cluster1/mychart/values.yaml",
		"cluster-charts/cluster1/myotherchart/values.yaml",
		"cluster-charts/cluster2/values.yaml",
		"some-path/values.yaml",
		"some-path/staging/values.yaml",
		"cluster-config/engineering/production/config.json",
		"cluster-config/engineering/dev/config.json",
		"p1/p2/config.json",
		"p1/app2/config.json",
		"p1/app3/config.json",
		"p1/config.json",
	}
	for _, file := range files {
		dir := filepath.Dir(file)
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, dir), 0o755))
		_, err := os.Create(filepath.Join(tmpDir, file))
		require.NoError(t, err)
	}
	require.NoError(t, runCmd(ctx, tmpDir, "git", "add", "."))
	require.NoError(t, runCmd(ctx, tmpDir, "git", "commit", "-m", "Initial commit"))

	tests := []struct {
		name                 string
		pattern              string
		isNewGlobbingEnabled bool
		expected             []string
	}{
		{
			name:                 "**/config.json (isNewGlobbingEnabled)",
			pattern:              "**/config.json",
			isNewGlobbingEnabled: true,
			expected: []string{
				"cluster-config/engineering/production/config.json",
				"cluster-config/engineering/dev/config.json",
				"p1/config.json",
				"p1/p2/config.json",
				"p1/app2/config.json",
				"p1/app3/config.json",
			},
		},
		{
			name:                 "**/config.json (non-isNewGlobbingEnabled)",
			pattern:              "**/config.json",
			isNewGlobbingEnabled: false,
			expected: []string{
				"cluster-config/engineering/production/config.json",
				"cluster-config/engineering/dev/config.json",
				"p1/config.json",
				"p1/p2/config.json",
				"p1/app2/config.json",
				"p1/app3/config.json",
			},
		},
		{
			name:                 "some-path/*.yaml (isNewGlobbingEnabled)",
			pattern:              "some-path/*.yaml",
			isNewGlobbingEnabled: true,
			expected:             []string{"some-path/values.yaml"},
		},
		{
			name:                 "some-path/*.yaml (non-isNewGlobbingEnabled)",
			pattern:              "some-path/*.yaml",
			isNewGlobbingEnabled: false,
			expected: []string{
				"some-path/values.yaml",
				"some-path/staging/values.yaml",
			},
		},
		{
			name:                 "p1/**/config.json (isNewGlobbingEnabled)",
			pattern:              "p1/**/config.json",
			isNewGlobbingEnabled: true,
			expected: []string{
				"p1/config.json",
				"p1/p2/config.json",
				"p1/app2/config.json",
				"p1/app3/config.json",
			},
		},
		{
			name:                 "p1/**/config.json (non-isNewGlobbingEnabled)",
			pattern:              "p1/**/config.json",
			isNewGlobbingEnabled: false,
			expected: []string{
				"p1/p2/config.json",
				"p1/app2/config.json",
				"p1/app3/config.json",
			},
		},
		{
			name:                 "cluster-config/**/config.json (isNewGlobbingEnabled)",
			pattern:              "cluster-config/**/config.json",
			isNewGlobbingEnabled: true,
			expected: []string{
				"cluster-config/engineering/production/config.json",
				"cluster-config/engineering/dev/config.json",
			},
		},
		{
			name:                 "cluster-config/**/config.json (isNewGlobbingEnabled=false)",
			pattern:              "cluster-config/**/config.json",
			isNewGlobbingEnabled: false,
			expected: []string{
				"cluster-config/engineering/dev/config.json",
				"cluster-config/engineering/production/config.json",
			},
		},
		{
			name:                 "cluster-config/*/dev/config.json (isNewGlobbingEnabled)",
			pattern:              "cluster-config/*/dev/config.json",
			isNewGlobbingEnabled: true,
			expected:             []string{"cluster-config/engineering/dev/config.json"},
		},
		{
			name:                 "cluster-config/*/dev/config.json (isNewGlobbingEnabled=false)",
			pattern:              "cluster-config/*/dev/config.json",
			isNewGlobbingEnabled: false,
			expected:             []string{"cluster-config/engineering/dev/config.json"},
		},
		{
			name:                 "cluster-charts/*/*/values.yaml (isNewGlobbingEnabled)",
			pattern:              "cluster-charts/*/*/values.yaml",
			isNewGlobbingEnabled: true,
			expected: []string{
				"cluster-charts/cluster1/mychart/values.yaml",
				"cluster-charts/cluster1/myotherchart/values.yaml",
			},
		},
		{
			name:                 "cluster-charts/*/*/values.yaml (isNewGlobbingEnabled=false)",
			pattern:              "cluster-charts/*/*/values.yaml",
			isNewGlobbingEnabled: false,
			expected: []string{
				"cluster-charts/cluster1/mychart/values.yaml",
				"cluster-charts/cluster1/myotherchart/values.yaml",
				"cluster-charts/cluster1/mychart/charts/mysubchart/values.yaml",
			},
		},
		{
			name:                 "cluster-charts/*/values.yaml (isNewGlobbingEnabled)",
			pattern:              "cluster-charts/*/values.yaml",
			isNewGlobbingEnabled: true,
			expected: []string{
				"cluster-charts/cluster2/values.yaml",
			},
		},
		{
			name:                 "cluster-charts/*/values.yaml (non-isNewGlobbingEnabled)",
			pattern:              "cluster-charts/*/values.yaml",
			isNewGlobbingEnabled: false,
			expected: []string{
				"cluster-charts/cluster2/values.yaml",
				"cluster-charts/cluster1/mychart/values.yaml",
				"cluster-charts/cluster1/myotherchart/values.yaml",
				"cluster-charts/cluster1/mychart/charts/mysubchart/values.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lsResult, err := client.LsFiles(tt.pattern, tt.isNewGlobbingEnabled)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.expected, lsResult)
		})
	}
}

func TestAnnotatedTagHandling(t *testing.T) {
	dir := t.TempDir()

	client, err := NewClientExt("https://github.com/argoproj/argo-cd.git", dir, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	// Test annotated tag resolution
	commitSHA, err := client.LsRemote("v1.0.0") // Known annotated tag
	require.NoError(t, err)

	// Verify we get commit SHA, not tag SHA
	assert.True(t, IsCommitSHA(commitSHA))

	// Test tag reference handling
	refs, err := client.LsRefs()
	require.NoError(t, err)

	// Verify tag exists in the list and points to a valid commit SHA
	assert.Contains(t, refs.Tags, "v1.0.0", "Tag v1.0.0 should exist in refs")
}

// Partial Clone / Sparse Checkout Tests

func TestComputePathHash(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected string
	}{
		{
			name:     "Single path",
			paths:    []string{"app/deployment"},
			expected: "0e7c69d8133a98d2",
		},
		{
			name:     "Multiple paths sorted",
			paths:    []string{"app", "deployment", "helm"},
			expected: "b8305263b1960b6c",
		},
		{
			name:     "Multiple paths unsorted (should produce same hash as sorted)",
			paths:    []string{"helm", "app", "deployment"},
			expected: "b8305263b1960b6c", // Same as sorted
		},
		{
			name:     "Empty paths",
			paths:    []string{},
			expected: "",
		},
		{
			name:     "Nil paths",
			paths:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := ComputePathHash(tt.paths)
			// Just verify it's a valid hex string and consistent
			if len(tt.paths) > 0 {
				assert.NotEmpty(t, hash)
				assert.Len(t, hash, 16)
				assert.Regexp(t, "^[a-f0-9]{16}$", hash)
				assert.Equal(t, tt.expected, hash)

				// Verify it's deterministic
				hash2 := ComputePathHash(tt.paths)
				assert.Equal(t, hash, hash2, "Hash should be deterministic")
			} else {
				assert.Empty(t, hash)
			}
		})
	}
}

func TestComputePathHashDeterministic(t *testing.T) {
	// Verify that the same paths in different orders produce the same hash
	paths1 := []string{"app", "deployment", "helm", "kustomize"}
	paths2 := []string{"kustomize", "helm", "deployment", "app"}
	paths3 := []string{"deployment", "app", "kustomize", "helm"}

	hash1 := ComputePathHash(paths1)
	hash2 := ComputePathHash(paths2)
	hash3 := ComputePathHash(paths3)

	assert.Equal(t, hash1, hash2, "Hashes should be equal regardless of order")
	assert.Equal(t, hash2, hash3, "Hashes should be equal regardless of order")
}

func TestSparseCheckoutConfiguration(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := t.Context()

	client, err := NewClientExt("", tmpDir, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	// Initialize a git repo
	require.NoError(t, runCmd(ctx, tmpDir, "git", "init"))

	// Create directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "app1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "app2"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "common"), 0o755))

	// Create files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app1", "config.yaml"), []byte("app1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "app2", "config.yaml"), []byte("app2"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "common", "shared.yaml"), []byte("common"), 0o644))

	// Commit files
	require.NoError(t, runCmd(ctx, tmpDir, "git", "add", "."))
	require.NoError(t, runCmd(ctx, tmpDir, "git", "commit", "-m", "Initial commit"))

	// Test configuring sparse checkout
	sparsePaths := []string{"app1", "common"}
	err = client.ConfigureSparseCheckout(sparsePaths)
	require.NoError(t, err)

	// Verify sparse-checkout file was created
	sparseCheckoutFile := filepath.Join(tmpDir, ".git", "info", "sparse-checkout")
	assert.FileExists(t, sparseCheckoutFile)

	// Read and verify sparse-checkout contents
	content, err := os.ReadFile(sparseCheckoutFile)
	require.NoError(t, err)
	contentStr := string(content)
	assert.Contains(t, contentStr, "app1")
	assert.Contains(t, contentStr, "common")
	assert.NotContains(t, contentStr, "app2")
}

func TestPartialCloneFetch(t *testing.T) {
	// This test requires a real git repository with partial clone support
	// We'll use the ArgoCD repo as it's publicly accessible
	tmpDir := t.TempDir()

	client, err := NewClientExt("https://github.com/argoproj/argo-cd.git", tmpDir, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	// Configure sparse checkout for a specific path
	sparsePaths := []string{"cmd/argocd"}
	err = client.ConfigureSparseCheckout(sparsePaths)
	require.NoError(t, err)

	// Perform partial fetch using consolidated Fetch method
	err = client.Fetch("HEAD", 0, true)
	require.NoError(t, err)

	// Verify that the repo was cloned with partial clone
	// Check for filter configuration
	output, err := runCmdOutput(t.Context(), tmpDir, "git", "config", "remote.origin.promisor")
	if err == nil {
		// If promisor is configured, verify it's true
		assert.Contains(t, string(output), "true")
	}
}

func TestSparseCheckoutWithMultiplePaths(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := t.Context()

	client, err := NewClientExt("", tmpDir, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	// Initialize a git repo
	require.NoError(t, runCmd(ctx, tmpDir, "git", "init"))

	// Create complex directory structure
	dirs := []string{
		"app1/deployment",
		"app1/service",
		"app2/deployment",
		"app2/service",
		"shared/common",
		"docs",
	}
	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, dir), 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, dir, "file.yaml"),
			[]byte(dir),
			0o644,
		))
	}

	// Commit files
	require.NoError(t, runCmd(ctx, tmpDir, "git", "add", "."))
	require.NoError(t, runCmd(ctx, tmpDir, "git", "commit", "-m", "Initial commit"))

	// Test configuring sparse checkout with multiple paths
	sparsePaths := []string{"app1", "shared/common"}
	err = client.ConfigureSparseCheckout(sparsePaths)
	require.NoError(t, err)

	// Verify sparse-checkout file was created with all paths
	sparseCheckoutFile := filepath.Join(tmpDir, ".git", "info", "sparse-checkout")
	content, err := os.ReadFile(sparseCheckoutFile)
	require.NoError(t, err)

	contentStr := string(content)
	for _, path := range sparsePaths {
		assert.Contains(t, contentStr, path, "Sparse checkout file should contain path: %s", path)
	}

	// Verify paths not in sparse list are not included
	assert.NotContains(t, contentStr, "app2")
	assert.NotContains(t, contentStr, "docs")
}

func TestNormalizeGitURLConsistency(t *testing.T) {
	// Test that normalizing the same URL multiple times produces consistent results
	urls := []string{
		"https://github.com/argoproj/argo-cd.git",
		"git@github.com:argoproj/argo-cd.git",
		"ssh://git@github.com/argoproj/argo-cd.git",
	}

	for _, url := range urls {
		normalized1 := NormalizeGitURL(url)
		normalized2 := NormalizeGitURL(url)
		assert.Equal(t, normalized1, normalized2, "Normalization should be consistent for: %s", url)
		assert.NotEmpty(t, normalized1, "Normalized URL should not be empty")
	}
}

func TestSparseCheckoutEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := t.Context()

	client, err := NewClientExt("", tmpDir, NopCreds{}, false, false, "", "")
	require.NoError(t, err)

	// Initialize a git repo
	require.NoError(t, runCmd(ctx, tmpDir, "git", "init"))

	// Test configuring sparse checkout with empty paths (should be error)
	err = client.ConfigureSparseCheckout([]string{})
	assert.Error(t, err)
}

// Helper function to run a command and get output
func runCmdOutput(ctx context.Context, workDir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	return cmd.Output()
}

// Test_nativeGitClient_Fetch_Combinations tests all combinations of fetch parameters
func Test_nativeGitClient_Fetch_Combinations(t *testing.T) {
	tests := []struct {
		name            string
		usePartialClone bool
		depth           int64
		description     string
	}{
		{
			name:            "Full clone (no partial, no depth)",
			usePartialClone: false,
			depth:           0,
			description:     "Should fetch all history with all blobs using --tags",
		},
		{
			name:            "Shallow clone only",
			usePartialClone: false,
			depth:           10,
			description:     "Should fetch limited history (10 commits) with all blobs using --depth",
		},
		{
			name:            "Partial clone only",
			usePartialClone: true,
			depth:           0,
			description:     "Should fetch all history but no blobs using --filter=blob:none",
		},
		{
			name:            "Both partial and shallow",
			usePartialClone: true,
			depth:           10,
			description:     "Should fetch limited history (10 commits) with no blobs using both --filter and --depth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			tempDir, err := _createEmptyGitRepo(ctx)
			require.NoError(t, err)

			// Add some commits to the origin repo to test depth
			for i := 0; i < 15; i++ {
				err = runCmd(ctx, tempDir, "git", "commit", "--allow-empty", "-m", fmt.Sprintf("Commit %d", i))
				require.NoError(t, err)
			}

			// Create a client for a different directory
			clientDir := t.TempDir()
			client, err := NewClientExt("file://"+tempDir, clientDir, NopCreds{}, true, false, "", "")
			require.NoError(t, err)

			err = client.Init()
			require.NoError(t, err)

			// Perform fetch with specified parameters
			err = client.Fetch("", tt.depth, tt.usePartialClone)
			require.NoError(t, err, tt.description)

			// Verify fetch succeeded by checking we can get HEAD
			commitSHA, err := client.LsRemote("HEAD")
			require.NoError(t, err)
			assert.True(t, IsCommitSHA(commitSHA))

			// If depth is specified, verify shallow clone was created
			if tt.depth > 0 {
				// Check if it's a shallow repository
				shallowFile := filepath.Join(clientDir, ".git", "shallow")
				_, err := os.Stat(shallowFile)
				// Shallow file should exist for shallow clones
				require.NoError(t, err, "Expected shallow file to exist for depth-limited fetch")
			}

			// If partial clone is specified, verify promisor remote was configured
			if tt.usePartialClone {
				output, err := runCmdOutput(ctx, clientDir, "git", "config", "remote.origin.promisor")
				if err == nil {
					assert.Contains(t, string(output), "true", "Expected promisor to be configured for partial clone")
				}
			}
		})
	}
}

// Test_nativeGitClient_Fetch_PartialClone_WithCheckout tests that partial clone works with checkout
func Test_nativeGitClient_Fetch_PartialClone_WithCheckout(t *testing.T) {
	ctx := t.Context()
	tempDir, err := _createEmptyGitRepo(ctx)
	require.NoError(t, err)

	// Create a file in the origin repo
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0o644)
	require.NoError(t, err)

	err = runCmd(ctx, tempDir, "git", "add", "test.txt")
	require.NoError(t, err)

	err = runCmd(ctx, tempDir, "git", "commit", "-m", "Add test file")
	require.NoError(t, err)

	// Create a client with partial clone
	clientDir := t.TempDir()
	client, err := NewClientExt("file://"+tempDir, clientDir, NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	// Fetch with partial clone
	err = client.Fetch("", 0, true)
	require.NoError(t, err)

	// Checkout should trigger blob download on demand
	commitSHA, err := client.LsRemote("HEAD")
	require.NoError(t, err)

	_, err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// Verify file was checked out (blob downloaded on demand)
	checkedOutFile := filepath.Join(clientDir, "test.txt")
	content, err := os.ReadFile(checkedOutFile)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content), "File should be checked out despite partial clone")
}

// Test_nativeGitClient_Fetch_ShallowAndPartial_Together tests combining both options
func Test_nativeGitClient_Fetch_ShallowAndPartial_Together(t *testing.T) {
	ctx := t.Context()
	tempDir, err := _createEmptyGitRepo(ctx)
	require.NoError(t, err)

	// Create multiple commits with files
	for i := 0; i < 20; i++ {
		testFile := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		err = os.WriteFile(testFile, []byte(fmt.Sprintf("content %d", i)), 0o644)
		require.NoError(t, err)

		err = runCmd(ctx, tempDir, "git", "add", fmt.Sprintf("file%d.txt", i))
		require.NoError(t, err)

		err = runCmd(ctx, tempDir, "git", "commit", "-m", fmt.Sprintf("Add file %d", i))
		require.NoError(t, err)
	}

	// Create a client with both shallow and partial clone
	clientDir := t.TempDir()
	client, err := NewClientExt("file://"+tempDir, clientDir, NopCreds{}, true, false, "", "")
	require.NoError(t, err)

	err = client.Init()
	require.NoError(t, err)

	// Fetch with both depth and partial clone
	depth := int64(5)
	err = client.Fetch("", depth, true)
	require.NoError(t, err)

	// Verify it's both shallow and partial
	shallowFile := filepath.Join(clientDir, ".git", "shallow")
	_, err = os.Stat(shallowFile)
	require.NoError(t, err, "Expected shallow file for depth-limited fetch")

	output, err := runCmdOutput(ctx, clientDir, "git", "config", "remote.origin.promisor")
	if err == nil {
		assert.Contains(t, string(output), "true", "Expected promisor for partial clone")
	}

	// Verify checkout still works
	commitSHA, err := client.LsRemote("HEAD")
	require.NoError(t, err)

	_, err = client.Checkout(commitSHA, false)
	require.NoError(t, err)

	// Latest file should exist
	latestFile := filepath.Join(clientDir, "file19.txt")
	content, err := os.ReadFile(latestFile)
	require.NoError(t, err)
	assert.Equal(t, "content 19", string(content))
}
