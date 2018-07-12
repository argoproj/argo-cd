package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCommitSHA(t *testing.T) {
	assert.True(t, IsCommitSHA("9d921f65f3c5373b682e2eb4b37afba6592e8f8b"))
	assert.True(t, IsCommitSHA("9D921F65F3C5373B682E2EB4B37AFBA6592E8F8B"))
	assert.False(t, IsCommitSHA("gd921f65f3c5373b682e2eb4b37afba6592e8f8b"))
	assert.False(t, IsCommitSHA("master"))
	assert.False(t, IsCommitSHA("HEAD"))
	assert.False(t, IsCommitSHA("9d921f6")) // only consider 40 characters hex strings as a commit-sha
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

func TestEnsureSuffix(t *testing.T) {
	data := [][]string{
		{"hello", "world", "helloworld"},
		{"helloworld", "world", "helloworld"},
		{"repo", ".git", "repo.git"},
		{"repo.git", ".git", "repo.git"},
		{"", "repo.git", "repo.git"},
		{"argo", "cd", "argocd"},
		{"argocd", "cd", "argocd"},
		{"argocd", "", "argocd"},
		{"", "argocd", "argocd"},
	}
	for _, table := range data {
		result := ensureSuffix(table[0], table[1])
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
		assert.Equal(t, v, IsSSHURL(k))
	}
}

func TestNormalizeUrl(t *testing.T) {
	data := map[string]string{
		"git@GITHUB.com:argoproj/test":           "git@github.com:argoproj/test.git",
		"git@GITHUB.com:argoproj/test.git":       "git@github.com:argoproj/test.git",
		"git@GITHUB.com:test":                    "git@github.com:test.git",
		"git@GITHUB.com:test.git":                "git@github.com:test.git",
		"https://GITHUB.com/argoproj/test":       "https://github.com/argoproj/test.git",
		"https://GITHUB.com/argoproj/test.git":   "https://github.com/argoproj/test.git",
		"https://github.com/TEST":                "https://github.com/TEST.git",
		"https://github.com/TEST.git":            "https://github.com/TEST.git",
		"ssh://git@GITHUB.com:argoproj/test":     "git@github.com:argoproj/test.git",
		"ssh://git@GITHUB.com:argoproj/test.git": "git@github.com:argoproj/test.git",
		"ssh://git@GITHUB.com:test.git":          "git@github.com:test.git",
		"ssh://git@github.com:test":              "git@github.com:test.git",
	}
	for k, v := range data {
		assert.Equal(t, v, NormalizeGitURL(k))
	}
}
