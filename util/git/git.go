package git

import (
	"regexp"
)

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
