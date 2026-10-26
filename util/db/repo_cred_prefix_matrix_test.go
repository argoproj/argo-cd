package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

// prefixCase is one raw credential-template URL against one raw Application
// repo URL. Both strings are passed through getRepositoryCredentialIndex,
// which selects repo credentials. expected is what that check does today.
type prefixCase struct {
	intent   string
	pattern  string
	repoURL  string
	expected bool
}

func prefixMatches(pattern, repoURL string) bool {
	creds := []*corev1.Secret{{
		Data: map[string][]byte{"url": []byte(pattern)},
	}}
	return (&secretsRepositoryBackend{}).getRepositoryCredentialIndex(creds, repoURL) == 0
}

func TestRepoCredPrefixMatrix(t *testing.T) {
	t.Parallel()

	cases := []prefixCase{
		{
			intent:   "Credential for this exact repository",
			pattern:  "https://github.com/argoproj/argo-cd",
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: true,
		},
		{
			intent:   "Credential for this exact repository; a trailing .git on the Application URL is the same repo",
			pattern:  "https://github.com/argoproj/argo-cd",
			repoURL:  "https://github.com/argoproj/argo-cd.git",
			expected: true,
		},
		{
			intent:   "Credential for this exact repository; a trailing .git on the template is the same repo",
			pattern:  "https://github.com/argoproj/argo-cd.git",
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: true,
		},
		{
			intent:   "Credential for this exact repository; surrounding whitespace on the template is ignored",
			pattern:  "  https://github.com/argoproj/argo-cd  ",
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: true,
		},
		{
			intent:   "Credential for this exact repository; surrounding whitespace on the Application URL is ignored",
			pattern:  "https://github.com/argoproj/argo-cd",
			repoURL:  "  https://github.com/argoproj/argo-cd  ",
			expected: true,
		},
		{
			intent:   "Credential for a different repository in the same organization",
			pattern:  "https://github.com/argoproj/argo-cd",
			repoURL:  "https://github.com/argoproj/argo-rollouts",
			expected: false,
		},

		{
			intent:   "Credential for repositories inside the argoproj organization",
			pattern:  "https://github.com/argoproj",
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: true,
		},
		{
			intent:   "Credential for repositories inside the argoproj organization; the trailing slash is the org boundary",
			pattern:  "https://github.com/argoproj/",
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: true,
		},
		{
			intent:   "Trailing slash keeps the credential inside argoproj and out of argoproj-labs",
			pattern:  "https://github.com/argoproj/",
			repoURL:  "https://github.com/argoproj-labs/applicationset",
			expected: false,
		},
		{
			intent:   "Trailing slash does not match a longer path segment that contains @",
			pattern:  "https://github.com/argoproj/",
			repoURL:  "https://github.com/argoproj@evil.com/malicious",
			expected: false,
		},
		{
			intent:   "Trailing slash is the directory boundary, so the org URL without the slash does not match",
			pattern:  "https://github.com/argoproj/",
			repoURL:  "https://github.com/argoproj",
			expected: false,
		},

		{
			intent:   "Credential for chart repos whose path continues team-",
			pattern:  "https://charts.example.com/team-",
			repoURL:  "https://charts.example.com/team-payments/chart",
			expected: true,
		},
		{
			intent:   "Credential for the team- prefix only; the team directory is a different path",
			pattern:  "https://charts.example.com/team-",
			repoURL:  "https://charts.example.com/team/chart",
			expected: false,
		},

		{
			intent:   "Credential for the my-company host does not include not-my-company.com",
			pattern:  "https://my-company",
			repoURL:  "https://not-my-company.com/repo",
			expected: false,
		},

		{
			intent:   "Credential bound to this token userinfo, for repos under my-org",
			pattern:  "https://x-access-token@github.com/my-org/",
			repoURL:  "https://x-access-token@github.com/my-org/app",
			expected: true,
		},
		{
			intent:   "Credential bound to this token userinfo does not apply when the Application URL omits it",
			pattern:  "https://x-access-token@github.com/my-org/",
			repoURL:  "https://github.com/my-org/app",
			expected: false,
		},
		{
			intent:   "Credential bound to this token userinfo does not apply to a different user",
			pattern:  "https://x-access-token@github.com/my-org/",
			repoURL:  "https://other-token@github.com/my-org/app",
			expected: false,
		},
		{
			intent:   "Credential for repositories on my-domain.com",
			pattern:  "https://my-domain.com",
			repoURL:  "https://my-domain.com/org/repo",
			expected: true,
		},

		{
			intent:   "SSH credential for repositories in the argoproj org",
			pattern:  "git@github.com:argoproj",
			repoURL:  "git@github.com:argoproj/argo-cd.git",
			expected: true,
		},
		{
			intent:   "SSH credential for the argoproj org also matches the ssh:// form of the same URL",
			pattern:  "git@github.com:argoproj",
			repoURL:  "ssh://git@github.com/argoproj/argo-cd",
			expected: true,
		},
		{
			// git@ constrains the user and the SSH form. Not covering HTTPS of the
			// same repo is a conservative miss and is accepted.
			intent:   "SSH credential for the argoproj org does not apply to the HTTPS URL of that repo",
			pattern:  "git@github.com:argoproj",
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: false,
		},
		{
			intent:   "SSH credential with a trailing slash stays inside the argoproj org",
			pattern:  "git@github.com:argoproj/",
			repoURL:  "ssh://git@github.com/argoproj/argo-cd",
			expected: true,
		},
		{
			intent:   "SSH credential with a trailing slash does not include argoproj-labs",
			pattern:  "git@github.com:argoproj/",
			repoURL:  "git@github.com:argoproj-labs/foo",
			expected: false,
		},
		{
			intent:   "Credential for repos on git.example.com port 8443",
			pattern:  "https://git.example.com:8443/org",
			repoURL:  "https://git.example.com:8443/org/repo",
			expected: true,
		},
		{
			intent:   "Credential for port 8443 does not apply to port 9443",
			pattern:  "https://git.example.com:8443/org",
			repoURL:  "https://git.example.com:9443/org/repo",
			expected: false,
		},
		{
			intent:   "A prefix that includes port 8443 does not cover the default port",
			pattern:  "https://git.example.com:8443/org",
			repoURL:  "https://git.example.com/org/repo",
			expected: false,
		},

		{
			intent:   "Credential for repositories under this registry path",
			pattern:  "registry.example.com:5000/team",
			repoURL:  "registry.example.com:5000/team/chart",
			expected: true,
		},
		{
			// A schemeless registry prefix not covering oci:// is a conservative miss.
			intent:   "Only schemeless registry URLs with this prefix, not oci://",
			pattern:  "registry.example.com:5000/team",
			repoURL:  "oci://registry.example.com:5000/team/chart",
			expected: false,
		},

		{
			intent:   "Credential for local repos under this directory",
			pattern:  "file:///var/git/team",
			repoURL:  "file:///var/git/team/app",
			expected: true,
		},
		{
			intent:   "Credential for local repos under this filesystem path",
			pattern:  "/var/git/team",
			repoURL:  "/var/git/team/app",
			expected: true,
		},
		{
			intent:   "This repository; a query string does not make it a different repo",
			pattern:  "https://github.com/org/repo",
			repoURL:  "https://github.com/org/repo?ref=main",
			expected: true,
		},
		{
			intent:   "This repository; a fragment does not make it a different repo",
			pattern:  "https://github.com/org/repo",
			repoURL:  "https://github.com/org/repo#readme",
			expected: true,
		},
		{
			intent:   "Credential prefix contains a literal asterisk and does not wildcard-match argo-cd",
			pattern:  "https://github.com/argoproj/*",
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: false,
		},
		{
			intent:   "Credential prefix contains a literal asterisk and matches a URL that contains that asterisk",
			pattern:  "https://github.com/argoproj/*",
			repoURL:  "https://github.com/argoproj/*/child",
			expected: true,
		},
		{
			intent:   "Credential prefix contains a literal question mark, not a single-character wildcard",
			pattern:  "https://github.com/org/repo-v?",
			repoURL:  "https://github.com/org/repo-v1",
			expected: false,
		},

		{
			intent:   "An unparseable template must not match an unrelated repository",
			pattern:  "https://{github,gitlab}.com/myorg",
			repoURL:  "https://bitbucket.org/other/app",
			expected: false,
		},
		{
			intent:   "Character class in the host does not include git-d",
			pattern:  "https://git-[abc].example.com/org",
			repoURL:  "https://git-d.example.com/org/repo",
			expected: false,
		},

		{
			intent:   "ssh:// with an scp colon does not cover a different org written the same way",
			pattern:  "ssh://git@github.com:argoproj",
			repoURL:  "ssh://git@github.com:evil/other",
			expected: false,
		},
		{
			intent:   "Azure ssh:// URL written with the scp colon does not cover a different project",
			pattern:  "ssh://git@ssh.dev.azure.com:v3/org/project/repo",
			repoURL:  "ssh://git@ssh.dev.azure.com:v3/other/project/repo",
			expected: false,
		},

		{
			intent:   "Credential for repositories on IPv6 localhost under /repo",
			pattern:  "https://[::1]/repo",
			repoURL:  "https://[::1]/repo/app",
			expected: true,
		},
		{
			intent:   "Credential for repositories on IPv6 localhost port 8443",
			pattern:  "https://[::1]:8443/repo",
			repoURL:  "https://[::1]:8443/repo/app",
			expected: true,
		},
		{
			intent:   "Credential for repositories on localhost",
			pattern:  "https://localhost/org/",
			repoURL:  "https://localhost/org/repo",
			expected: true,
		},
		{
			intent:   "Every HTTPS repository, and not HTTP",
			pattern:  "https://",
			repoURL:  "https://github.com/org/repo",
			expected: true,
		},
		{
			intent:   "Every HTTPS repository does not include an HTTP URL",
			pattern:  "https://",
			repoURL:  "http://github.com/org/repo",
			expected: false,
		},

		{
			intent:   "Hostname case does not distinguish the repository",
			pattern:  "https://GitHub.com/argoproj/argo-cd",
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.intent, func(t *testing.T) {
			t.Parallel()
			got := prefixMatches(tc.pattern, tc.repoURL)
			assert.Equal(t, tc.expected, got, "intent: %s\npattern: %q\nrepoURL: %q", tc.intent, tc.pattern, tc.repoURL)
		})
	}
}

// longestCase feeds several raw credential templates and one raw Application
// repo URL through the same longest-prefix selection.
type longestCase struct {
	intent   string
	patterns []string
	repoURL  string
	expected string
}

func winningPrefix(patterns []string, repoURL string) string {
	creds := make([]*corev1.Secret, len(patterns))
	for i, pattern := range patterns {
		creds[i] = &corev1.Secret{Data: map[string][]byte{"url": []byte(pattern)}}
	}
	idx := (&secretsRepositoryBackend{}).getRepositoryCredentialIndex(creds, repoURL)
	if idx < 0 {
		return ""
	}
	return patterns[idx]
}

func TestRepoCredPrefixLongestMatch(t *testing.T) {
	t.Parallel()

	cases := []longestCase{
		{
			intent:   "The longest matching template wins over a shorter prefix of the same URL",
			patterns: []string{"https://github.com/argoproj", "https://github.com/argoproj/argo-cd"},
			repoURL:  "https://github.com/argoproj/argo-cd.git",
			expected: "https://github.com/argoproj/argo-cd",
		},
		{
			intent:   "Definition order does not matter; the longest match still wins",
			patterns: []string{"https://github.com/argoproj/argo-cd", "https://github.com/argoproj"},
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: "https://github.com/argoproj/argo-cd",
		},
		{
			intent:   "A more specific template that does not match must not win",
			patterns: []string{"https://github.com/argoproj", "https://github.com/argoproj/argo-cd"},
			repoURL:  "https://github.com/argoproj/argo-rollouts",
			expected: "https://github.com/argoproj",
		},
		{
			intent:   "When both the org prefix and the trailing-slash form match, the longer one wins",
			patterns: []string{"https://github.com/argoproj", "https://github.com/argoproj/"},
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: "https://github.com/argoproj/",
		},
		{
			intent:   "When two templates normalize to the same length, the earlier one is kept",
			patterns: []string{"https://github.com/argoproj/argo-cd", "https://GitHub.com/argoproj/argo-cd"},
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: "https://github.com/argoproj/argo-cd",
		},
		{
			intent:   "Only the template that is actually a prefix is selected",
			patterns: []string{"https://example.com/a", "https://example.com/b"},
			repoURL:  "https://example.com/a/repo",
			expected: "https://example.com/a",
		},
		{
			intent:   "No credential applies when neither template is a prefix",
			patterns: []string{"https://example.com/a", "https://example.com/b"},
			repoURL:  "https://example.com/c/repo",
			expected: "",
		},
		{
			intent:   "An unparseable template must not override a real prefix",
			patterns: []string{"https://{github,gitlab}.com/org", "https://github.com/argoproj"},
			repoURL:  "https://github.com/argoproj/argo-cd",
			expected: "https://github.com/argoproj",
		},
	}

	for _, tc := range cases {
		t.Run(tc.intent, func(t *testing.T) {
			t.Parallel()
			got := winningPrefix(tc.patterns, tc.repoURL)
			assert.Equal(t, tc.expected, got, "intent: %s\npatterns: %#v\nrepoURL: %q", tc.intent, tc.patterns, tc.repoURL)
		})
	}
}
