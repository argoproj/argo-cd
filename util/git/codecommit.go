package git

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/env"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// codeCommitHostRegexp matches the HTTPS Git endpoints of AWS CodeCommit. The first capture group is the region.
var codeCommitHostRegexp = regexp.MustCompile(`^git-codecommit(?:-fips)?\.([a-z0-9-]+)\.amazonaws\.(?:com|com\.cn|eu)$`)

// codeCommitRetrieveTimeout bounds how long fetching AWS credentials may take for a single Git operation.
const codeCommitRetrieveTimeout = 30 * time.Second

var (
	// codeCommitProviders caches one AWS credentials provider per region, so that STS (IRSA) or the
	// Pod Identity agent is only contacted when the cached temporary credentials are about to expire.
	codeCommitProviders sync.Map // region -> aws.CredentialsProvider

	// codeCommitNow is the clock used for signing. It is a variable so tests can freeze it.
	codeCommitNow = time.Now
)

var _ Creds = CodeCommitCreds{}

// CodeCommitCreds authenticates to AWS CodeCommit over HTTPS with the AWS identity of the running
// process (IRSA, EKS Pod Identity, instance profile, ...). For every Git operation it derives a
// short-lived Git password from a SigV4 signature, the same scheme used by git-remote-codecommit
// and the AWS CLI credential helper, so no static credential is ever stored.
type CodeCommitCreds struct {
	host   string
	path   string
	region string
	store  CredsStore
}

// IsCodeCommitIAMEnabled returns whether this process may use its AWS identity to authenticate
// to AWS CodeCommit repositories that have no credentials configured.
func IsCodeCommitIAMEnabled() bool {
	return env.ParseBoolFromEnv(common.EnvGitCodeCommitIAMEnabled, false)
}

// NewCodeCommitCreds returns CodeCommit IAM credentials for repoURL. The second return value is false
// when the feature is disabled or repoURL is not an HTTPS AWS CodeCommit URL, in which case the
// credentials must not be used.
func NewCodeCommitCreds(repoURL string, store CredsStore) (CodeCommitCreds, bool) {
	if !IsCodeCommitIAMEnabled() {
		return CodeCommitCreds{}, false
	}
	host, path, region, ok := parseCodeCommitURL(repoURL)
	if !ok {
		return CodeCommitCreds{}, false
	}
	return CodeCommitCreds{host: host, path: path, region: region, store: store}, true
}

// parseCodeCommitURL returns the host, repository path and region of an HTTPS AWS CodeCommit URL.
// Credentials are only ever signed for, and sent to, a URL accepted by this function.
func parseCodeCommitURL(repoURL string) (host, path, region string, ok bool) {
	u, err := url.Parse(repoURL)
	if err != nil || u.Scheme != "https" {
		return "", "", "", false
	}
	host = strings.ToLower(u.Host)
	m := codeCommitHostRegexp.FindStringSubmatch(host)
	if m == nil {
		return "", "", "", false
	}
	path = strings.TrimSuffix(u.EscapedPath(), "/")
	if path == "" {
		return "", "", "", false
	}
	return host, path, m[1], true
}

// GetUserInfo returns empty values: the AWS identity carries no Git author information.
func (c CodeCommitCreds) GetUserInfo(_ context.Context) (string, string, error) {
	return "", "", nil
}

func (c CodeCommitCreds) Environ() (io.Closer, []string, error) {
	username, password, err := c.basicAuth()
	if err != nil {
		return NopCloser{}, nil, err
	}
	nonce := c.store.Add(username, password)
	env := c.store.Environ(nonce)
	return utilio.NewCloser(func() error {
		c.store.Remove(nonce)
		return nil
	}), env, nil
}

// basicAuth returns the username and short-lived password to use for HTTP basic authentication.
// The username is not URL-encoded: it is passed to Git through GIT_ASKPASS or go-git, not in a URL.
func (c CodeCommitCreds) basicAuth() (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), codeCommitRetrieveTimeout)
	defer cancel()

	provider, err := codeCommitCredentialsProvider(ctx, c.region)
	if err != nil {
		return "", "", err
	}
	awsCreds, err := provider.Retrieve(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to retrieve AWS credentials for CodeCommit in region %q: %w", c.region, err)
	}

	username := awsCreds.AccessKeyID
	if awsCreds.SessionToken != "" {
		username += "%" + awsCreds.SessionToken
	}
	return username, codeCommitPassword(awsCreds, c.host, c.path, c.region, codeCommitNow()), nil
}

func codeCommitCredentialsProvider(ctx context.Context, region string) (aws.CredentialsProvider, error) {
	if p, ok := codeCommitProviders.Load(region); ok {
		return p.(aws.CredentialsProvider), nil
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for CodeCommit in region %q: %w", region, err)
	}
	if cfg.Credentials == nil {
		return nil, fmt.Errorf("no AWS credentials available for CodeCommit in region %q", region)
	}
	provider := cfg.Credentials
	if _, cached := provider.(*aws.CredentialsCache); !cached {
		provider = aws.NewCredentialsCache(provider)
	}
	p, _ := codeCommitProviders.LoadOrStore(region, provider)
	return p.(aws.CredentialsProvider), nil
}

// codeCommitPassword computes the Git password AWS CodeCommit accepts for HTTPS access: the signing
// timestamp, "Z", and a SigV4 signature over a Git-specific canonical request. The AWS SDK signer
// cannot be used because it always signs extra X-Amz-* headers, which CodeCommit does not expect.
func codeCommitPassword(creds aws.Credentials, host, path, region string, t time.Time) string {
	ts := t.UTC().Format("20060102T150405")
	date := ts[:8]
	canonical := "GIT\n" + path + "\n\nhost:" + host + "\n\nhost\n"
	scope := date + "/" + region + "/codecommit/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + ts + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	k := hmacSHA256([]byte("AWS4"+creds.SecretAccessKey), date)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, "codecommit")
	k = hmacSHA256(k, "aws4_request")
	return ts + "Z" + hex.EncodeToString(hmacSHA256(k, toSign))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
