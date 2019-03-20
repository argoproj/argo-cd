package repos

import (
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// Client is a generic git client interface
type Client interface {
	// Return the directory of the checkout code. Aka "working tree".
	WorkDir() string
	// Test that we can connect to the remote repo.
	Test() error
	// Checkout out the specified revision of the code from the remote repo into the working tree.
	// revision can be empty string, which should be treated as HEAD/latest
	Checkout(path, revision string) (string, error)
	// Resolve a potentially ambiguous revision (e.g. tag or branch) into non-ambiguous revision.
	// revision can be empty string, which should be treated as HEAD/latest
	ResolveRevision(path, revision string) (string, error)
	// List files in the local working tree.
	LsFiles(path string) ([]string, error)
}

// ClientFactory is a factory of Clients
// Primarily used to support creation of mock git clients during unit testing
type ClientFactory interface {
	NewClient(c Config, workDir string) (Client, error)
}

type factory struct{}

func NewFactory() ClientFactory {
	return &factory{}
}

func (f factory) NewClient(c Config, workDir string) (Client, error) {
	if c.Type == "helm" {
		return f.newHelmClient(c.Url, c.Name, workDir, c.Username, c.Password, c.CAData, c.CertData, c.KeyData)
	} else {
		return f.newGitClient(c.Url, workDir, c.Username, c.Password, c.SSHPrivateKey, c.InsecureIgnoreHostKey)
	}
}

// EnsurePrefix idempotently ensures that a base string has a given prefix.
func ensurePrefix(s, prefix string) string {
	if !strings.HasPrefix(s, prefix) {
		s = prefix + s
	}
	return s
}

// removeSuffix idempotently removes a given suffix
func removeSuffix(s, suffix string) string {
	if strings.HasSuffix(s, suffix) {
		return s[0 : len(s)-len(suffix)]
	}
	return s
}

var commitSHARegex = regexp.MustCompile("^[0-9A-Fa-f]{40}$")

// IsCommitSHA returns whether or not a string is a 40 character SHA-1
func IsCommitSHA(sha string) bool {
	return commitSHARegex.MatchString(sha)
}

var truncatedCommitSHARegex = regexp.MustCompile("^[0-9A-Fa-f]{7,}$")

// IsTruncatedCommitSHA returns whether or not a string is a truncated  SHA-1
func IsTruncatedCommitSHA(sha string) bool {
	return truncatedCommitSHARegex.MatchString(sha)
}

// SameURL returns whether or not the two repository URLs are equivalent in location
func SameURL(leftRepo, rightRepo string) bool {
	return NormalizeURL(leftRepo) == NormalizeURL(rightRepo)
}

// NormalizeURL normalizes a git URL for purposes of comparison, as well as preventing redundant
// local clones (by normalizing various forms of a URL to a consistent location).
// Prefer using SameURL() over this function when possible. This algorithm may change over time
// and should not be considered stable from release to release
func NormalizeURL(repo string) string {
	repo = strings.ToLower(strings.TrimSpace(repo))
	if IsSSHURL(repo) {
		repo = ensurePrefix(repo, "ssh://")
	}
	repo = removeSuffix(repo, ".git")
	repoURL, err := url.Parse(repo)
	if err != nil {
		return ""
	}
	normalized := repoURL.String()
	return strings.TrimPrefix(normalized, "ssh://")
}

// IsSSHURL returns true if supplied URL is SSH URL
func IsSSHURL(url string) bool {
	return strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://")
}

// TestRepo tests if a repo exists and is accessible with the given credentials
func TestRepo(c Config) error {

	err := c.Validate()
	if err != nil {
		return err
	}

	tmp, err := ioutil.TempDir("", "repos")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	client, err := NewFactory().NewClient(c, tmp)
	if err != nil {
		return err
	}
	return client.Test()
}
