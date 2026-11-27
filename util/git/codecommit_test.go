package git

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
)

// Test vectors from aws/git-remote-codecommit (test/git_url_test.py, commit 80bad9b).
var (
	codeCommitTestTime  = time.Date(2017, 12, 24, 11, 53, 20, 0, time.UTC)
	codeCommitTestCreds = aws.Credentials{AccessKeyID: "access", SecretAccessKey: "secret"}
)

const codeCommitTestPassword = "20171224T115320Zb6df2d758a8023b2f000a546417007b65494f3ce8ad0300fd45fcfa173f1959a"

func TestCodeCommitPassword(t *testing.T) {
	tests := []struct {
		name  string
		creds aws.Credentials
		host  string
		time  time.Time
		want  string
	}{
		{
			name:  "upstream test_sign",
			creds: codeCommitTestCreds,
			host:  "git-codecommit.us-west-2.amazonaws.com",
			time:  codeCommitTestTime,
			want:  codeCommitTestPassword,
		},
		{
			name:  "upstream test_git_url_with_override",
			creds: codeCommitTestCreds,
			host:  "land.called.honahlee",
			time:  codeCommitTestTime,
			want:  "20171224T115320Za14a37a53035bbceb6e1748247b143124fe7326cdffd3d418236a592ce6158a0",
		},
		{
			name:  "session token is not part of the signature",
			creds: aws.Credentials{AccessKeyID: "access", SecretAccessKey: "secret", SessionToken: "token"},
			host:  "git-codecommit.us-west-2.amazonaws.com",
			time:  codeCommitTestTime,
			want:  codeCommitTestPassword,
		},
		{
			name:  "non-UTC time is signed in UTC",
			creds: codeCommitTestCreds,
			host:  "git-codecommit.us-west-2.amazonaws.com",
			time:  codeCommitTestTime.In(time.FixedZone("UTC-3", -3*3600)),
			want:  codeCommitTestPassword,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, codeCommitPassword(tt.creds, tt.host, "/v1/repos/test_repo", "us-west-2", tt.time))
		})
	}
}

func TestParseCodeCommitURL(t *testing.T) {
	tests := []struct {
		url        string
		ok         bool
		wantHost   string
		wantPath   string
		wantRegion string
	}{
		{url: "https://git-codecommit.us-east-2.amazonaws.com/v1/repos/config", ok: true, wantHost: "git-codecommit.us-east-2.amazonaws.com", wantPath: "/v1/repos/config", wantRegion: "us-east-2"},
		{url: "https://git-codecommit.us-east-2.amazonaws.com/v1/repos/config/", ok: true, wantHost: "git-codecommit.us-east-2.amazonaws.com", wantPath: "/v1/repos/config", wantRegion: "us-east-2"},
		{url: "https://GIT-CODECOMMIT.US-EAST-2.AMAZONAWS.COM/v1/repos/config", ok: true, wantHost: "git-codecommit.us-east-2.amazonaws.com", wantPath: "/v1/repos/config", wantRegion: "us-east-2"},
		{url: "https://git-codecommit-fips.us-gov-west-1.amazonaws.com/v1/repos/config", ok: true, wantHost: "git-codecommit-fips.us-gov-west-1.amazonaws.com", wantPath: "/v1/repos/config", wantRegion: "us-gov-west-1"},
		{url: "https://git-codecommit.cn-north-1.amazonaws.com.cn/v1/repos/config", ok: true, wantHost: "git-codecommit.cn-north-1.amazonaws.com.cn", wantPath: "/v1/repos/config", wantRegion: "cn-north-1"},
		{url: "https://git-codecommit.eusc-de-east-1.amazonaws.eu/v1/repos/config", ok: true, wantHost: "git-codecommit.eusc-de-east-1.amazonaws.eu", wantPath: "/v1/repos/config", wantRegion: "eusc-de-east-1"},
		{url: "http://git-codecommit.us-east-2.amazonaws.com/v1/repos/config"},
		{url: "ssh://git-codecommit.us-east-2.amazonaws.com/v1/repos/config"},
		{url: "git-codecommit.us-east-2.amazonaws.com:v1/repos/config"},
		{url: "https://git-codecommit.us-east-2.amazonaws.com"},
		{url: "https://git-codecommit.us-east-2.amazonaws.com:8443/v1/repos/config"},
		{url: "https://git-codecommit.us-east-1.amazonaws.com.example.com/v1/repos/config"},
		{url: "https://evil-git-codecommit.us-east-1.amazonaws.com/v1/repos/config"},
		{url: "https://github.com/argoproj/argo-cd"},
		{url: "https://user:pass@example.com/v1/repos/config"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			host, path, region, ok := parseCodeCommitURL(tt.url)
			require.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantPath, path)
			assert.Equal(t, tt.wantRegion, region)
		})
	}
}

func TestNewCodeCommitCreds(t *testing.T) {
	const repoURL = "https://git-codecommit.us-east-2.amazonaws.com/v1/repos/config"

	t.Run("disabled by default", func(t *testing.T) {
		t.Setenv(common.EnvGitCodeCommitIAMEnabled, "")
		_, ok := NewCodeCommitCreds(repoURL, NoopCredsStore{})
		assert.False(t, ok)
	})
	t.Run("enabled with a CodeCommit URL", func(t *testing.T) {
		t.Setenv(common.EnvGitCodeCommitIAMEnabled, "true")
		creds, ok := NewCodeCommitCreds(repoURL, NoopCredsStore{})
		require.True(t, ok)
		assert.Equal(t, CodeCommitCreds{host: "git-codecommit.us-east-2.amazonaws.com", path: "/v1/repos/config", region: "us-east-2", store: NoopCredsStore{}}, creds)
	})
	t.Run("enabled with a non-CodeCommit URL", func(t *testing.T) {
		t.Setenv(common.EnvGitCodeCommitIAMEnabled, "true")
		_, ok := NewCodeCommitCreds("https://github.com/argoproj/argo-cd", NoopCredsStore{})
		assert.False(t, ok)
	})
}

// useCodeCommitTestProvider makes CodeCommitCreds in region use provider and the fixed test clock.
func useCodeCommitTestProvider(t *testing.T, region string, provider aws.CredentialsProvider) {
	t.Helper()
	codeCommitProviders.Store(region, provider)
	codeCommitNow = func() time.Time { return codeCommitTestTime }
	t.Cleanup(func() {
		codeCommitProviders.Delete(region)
		codeCommitNow = time.Now
	})
}

func staticProvider(creds aws.Credentials) aws.CredentialsProvider {
	return aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) { return creds, nil })
}

type recordingCredsStore struct {
	username, password string
	removed            bool
}

func (s *recordingCredsStore) Add(username string, password string) string {
	s.username, s.password = username, password
	return "nonce"
}

func (s *recordingCredsStore) Remove(id string) {
	s.removed = id == "nonce"
}

func (s *recordingCredsStore) Environ(id string) []string {
	return []string{"NONCE=" + id}
}

func TestCodeCommitCreds_Environ(t *testing.T) {
	useCodeCommitTestProvider(t, "us-west-2", staticProvider(aws.Credentials{AccessKeyID: "access", SecretAccessKey: "secret", SessionToken: "token"}))
	store := &recordingCredsStore{}
	creds := CodeCommitCreds{host: "git-codecommit.us-west-2.amazonaws.com", path: "/v1/repos/test_repo", region: "us-west-2", store: store}

	closer, env, err := creds.Environ()
	require.NoError(t, err)
	assert.Equal(t, []string{"NONCE=nonce"}, env)
	// The username must not be URL-encoded: it reaches Git through GIT_ASKPASS, not a URL.
	assert.Equal(t, "access%token", store.username)
	assert.Equal(t, codeCommitTestPassword, store.password)

	require.NoError(t, closer.Close())
	assert.True(t, store.removed)
}

func TestCodeCommitCreds_EnvironProviderError(t *testing.T) {
	useCodeCommitTestProvider(t, "us-west-2", aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{}, errors.New("no identity")
	}))
	store := &recordingCredsStore{}
	creds := CodeCommitCreds{host: "git-codecommit.us-west-2.amazonaws.com", path: "/v1/repos/test_repo", region: "us-west-2", store: store}

	_, _, err := creds.Environ()
	require.ErrorContains(t, err, "no identity")
	assert.Empty(t, store.username)
}

func TestCodeCommitCreds_GetUserInfo(t *testing.T) {
	username, email, err := CodeCommitCreds{}.GetUserInfo(t.Context())
	require.NoError(t, err)
	assert.Empty(t, username)
	assert.Empty(t, email)
}

func TestNewAuth_CodeCommit(t *testing.T) {
	useCodeCommitTestProvider(t, "us-west-2", staticProvider(codeCommitTestCreds))
	creds := CodeCommitCreds{host: "git-codecommit.us-west-2.amazonaws.com", path: "/v1/repos/test_repo", region: "us-west-2", store: NoopCredsStore{}}

	auth, err := newAuth("https://git-codecommit.us-west-2.amazonaws.com/v1/repos/test_repo", creds)
	require.NoError(t, err)
	assert.Equal(t, &githttp.BasicAuth{Username: "access", Password: codeCommitTestPassword}, auth)
}

func TestCodeCommitCredentialsProvider_Cached(t *testing.T) {
	const region = "test-cache-region-1"
	t.Cleanup(func() { codeCommitProviders.Delete(region) })
	// Static credentials from the environment keep LoadDefaultConfig away from any real AWS identity.
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")

	p1, err := codeCommitCredentialsProvider(t.Context(), region)
	require.NoError(t, err)
	p2, err := codeCommitCredentialsProvider(t.Context(), region)
	require.NoError(t, err)
	assert.Same(t, p1, p2)

	creds, err := p1.Retrieve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "AKIDEXAMPLE", creds.AccessKeyID)
}
