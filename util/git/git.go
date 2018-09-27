package git

import (
	"net/url"
	"regexp"
	"strings"
)

// EnsurePrefix idempotently ensures that a base string has a given prefix.
func ensurePrefix(s, prefix string) string {
	if !strings.HasPrefix(s, prefix) {
		s = prefix + s
	}
	return s
}

// EnsureSuffix idempotently ensures that a base string has a given suffix.
func ensureSuffix(s, suffix string) string {
	if !strings.HasSuffix(s, suffix) {
		s += suffix
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

// NormalizeGitURL normalizes a git URL for lookup and storage
func NormalizeGitURL(repo string) string {
	// preprocess
	repo = strings.TrimSpace(repo)
	repo = ensureSuffix(repo, ".git")
	if IsSSHURL(repo) {
		repo = ensurePrefix(repo, "ssh://")
	}

	// process
	repoURL, err := url.Parse(repo)
	if err != nil {
		return ""
	}

	// postprocess
	repoURL.Host = strings.ToLower(repoURL.Host)
	normalized := repoURL.String()
	return strings.TrimPrefix(normalized, "ssh://")
}

// IsSSHURL returns true if supplied URL is SSH URL
func IsSSHURL(url string) bool {
	return strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://")
}

// TestRepo tests if a repo exists and is accessible with the given credentials
func TestRepo(repo, username, password string, sshPrivateKey string) error {
	clnt, err := NewFactory().NewClient(repo, "", username, password, sshPrivateKey)
	if err != nil {
		return err
	}
	_, err = clnt.LsRemote("HEAD")
	return err
}
