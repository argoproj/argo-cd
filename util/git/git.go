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

// removeSuffix idempotently removes a given suffix
func removeSuffix(s, suffix string) string {
	if strings.HasSuffix(s, suffix) {
		return s[0 : len(s)-len(suffix)]
	}
	return s
}

var (
	commitSHARegex = regexp.MustCompile("^[0-9A-Fa-f]{40}$")
	sshURLRegex    = regexp.MustCompile("^(ssh://)?([^/:]*?)@[^@]+$")
	httpsURLRegex  = regexp.MustCompile("^(https://).*")
)

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
	normalLeft := NormalizeGitURL(leftRepo)
	normalRight := NormalizeGitURL(rightRepo)
	return normalLeft != "" && normalRight != "" && normalLeft == normalRight
}

// NormalizeGitURL normalizes a git URL for purposes of comparison, as well as preventing redundant
// local clones (by normalizing various forms of a URL to a consistent location).
// Prefer using SameURL() over this function when possible. This algorithm may change over time
// and should not be considered stable from release to release
func NormalizeGitURL(repo string) string {
	repo = strings.ToLower(strings.TrimSpace(repo))
	if yes, _ := IsSSHURL(repo); yes {
		if !strings.HasPrefix(repo, "ssh://") {
			// We need to replace the first colon in git@server... style SSH URLs with a slash, otherwise
			// net/url.Parse will interpret it incorrectly as the port.
			repo = strings.Replace(repo, ":", "/", 1)
			repo = ensurePrefix(repo, "ssh://")
		}
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
func IsSSHURL(url string) (bool, string) {
	matches := sshURLRegex.FindStringSubmatch(url)
	if len(matches) > 2 {
		return true, matches[2]
	}
	return false, ""
}

// IsHTTPSURL returns true if supplied URL is HTTPS URL
func IsHTTPSURL(url string) bool {
	return httpsURLRegex.MatchString(url)
}

// TestRepo tests if a repo exists and is accessible with the given credentials
func TestRepo(repo string, creds Creds, insecure bool, enableLfs bool) error {
	clnt, err := NewClient(repo, creds, insecure, enableLfs)
	if err != nil {
		return err
	}
	_, err = clnt.LsRemote("HEAD")
	return err
}
