package webhook

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_GetWebUrlRegex(t *testing.T) {
	tests := []struct {
		shouldMatch bool
		webURL      string
		repo        string
		name        string
	}{
		// Ensure input is regex-escaped.
		{false, "https://example.com/org/a..d", "https://example.com/org/abcd", "dots in repo names should not be treated as wildcards"},
		{false, "https://an.example.com/org/repo", "https://an-example.com/org/repo", "dots in domain names should not be treated as wildcards"},

		// Standard cases.
		{true, "https://example.com/org/repo", "https://example.com/org/repo", "exact match should match"},
		{false, "https://example.com/org/repo", "https://example.com/org/repo-2", "partial match should not match"},
		{true, "https://example.com/org/repo", "https://example.com/org/repo.git", "no .git should match with .git"},
		{true, "https://example.com/org/repo", "git@example.com:org/repo", "git without protocol should match"},
		{true, "https://example.com/org/repo", "user@example.com:org/repo", "git with non-git username should match"},
		{true, "https://example.com/org/repo", "ssh://git@example.com/org/repo", "git with protocol should match"},
		{true, "https://example.com/org/repo", "ssh://git@example.com:22/org/repo", "git with port number should match"},
		{true, "https://example.com:443/org/repo", "ssh://git@example.com:22/org/repo", "https and ssh w/ different port numbers should match"},
		{true, "https://example.com:443/org/repo", "ssh://git@ssh.example.com:443/org/repo", "https and ssh w/ ssh subdomain should match"},
		{true, "https://example.com:443/org/repo", "ssh://git@altssh.example.com:443/org/repo", "https and ssh w/ altssh subdomain should match"},
		{false, "https://example.com:443/org/repo", "ssh://git@unknown.example.com:443/org/repo", "https and ssh w/ unknown subdomain should not match"},
		{true, "https://example.com/org/repo", "ssh://user-name@example.com/org/repo", "valid usernames with hyphens in repo should match"},
		{false, "https://example.com/org/repo", "ssh://-user-name@example.com/org/repo", "invalid usernames with hyphens in repo should not match"},
		{true, "https://example.com:443/org/repo", "GIT@EXAMPLE.COM:22:ORG/REPO", "matches aren't case-sensitive"},
		{true, "https://example.com/org/repo%20", "https://example.com/org/repo%20", "escape codes in path are preserved"},
	}
	for _, testCase := range tests {
		testCopy := testCase
		t.Run(testCopy.name, func(t *testing.T) {
			t.Parallel()
			regexp, err := GetWebUrlRegex(testCopy.webURL)
			require.NoError(t, err)
			if matches := regexp.MatchString(testCopy.repo); matches != testCopy.shouldMatch {
				t.Errorf("sourceRevisionHasChanged() = %v, want %v", matches, testCopy.shouldMatch)
			}
		})
	}
}
