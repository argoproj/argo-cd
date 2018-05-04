package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	}
	for _, table := range data {
		result := ensureSuffix(table[0], table[1])
		assert.Equal(t, result, table[2])
	}
}

func TestIsSSHUrl(t *testing.T) {
	data := map[string]bool{
		"git@GITHUB.com:argoproj/test.git":     true,
		"git@github.com:test.git":              true,
		"https://github.com/argoproj/test.git": false,
		"git://github.com/argoproj/test.git":   false,
	}
	for k, v := range data {
		assert.Equal(t, IsSshURL(k), v)
	}
}

func TestNormalizeUrl(t *testing.T) {
	data := map[string]string{
		"git@GITHUB.com:test.git": "git@github.com:test.git",
	}
	for k, v := range data {
		assert.Equal(t, NormalizeGitURL(k), v)
	}
}
