package repos

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestRemoveSuffix(t *testing.T) {
	data := [][]string{
		{"hello.git", ".git", "hello"},
		{"hello", ".git", "hello"},
		{".git", ".git", ""},
	}
	for _, table := range data {
		result := removeSuffix(table[0], table[1])
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

func TestSameURL(t *testing.T) {
	data := map[string]string{
		"git@GITHUB.com:argoproj/test":                     "git@github.com:argoproj/test.git",
		"git@GITHUB.com:argoproj/test.git":                 "git@github.com:argoproj/test.git",
		"git@GITHUB.com:test":                              "git@github.com:test.git",
		"git@GITHUB.com:test.git":                          "git@github.com:test.git",
		"https://GITHUB.com/argoproj/test":                 "https://github.com/argoproj/test.git",
		"https://GITHUB.com/argoproj/test.git":             "https://github.com/argoproj/test.git",
		"https://github.com/FOO":                           "https://github.com/foo",
		"https://github.com/TEST":                          "https://github.com/TEST.git",
		"https://github.com/TEST.git":                      "https://github.com/TEST.git",
		"ssh://git@GITHUB.com:argoproj/test":               "git@github.com:argoproj/test.git",
		"ssh://git@GITHUB.com:argoproj/test.git":           "git@github.com:argoproj/test.git",
		"ssh://git@GITHUB.com:test.git":                    "git@github.com:test.git",
		"ssh://git@github.com:test":                        "git@github.com:test.git",
		" https://github.com/argoproj/test ":               "https://github.com/argoproj/test.git",
		"\thttps://github.com/argoproj/test\n":             "https://github.com/argoproj/test.git",
		"https://1234.visualstudio.com/myproj/_git/myrepo": "https://1234.visualstudio.com/myproj/_git/myrepo",
		"https://dev.azure.com/1234/myproj/_git/myrepo":    "https://dev.azure.com/1234/myproj/_git/myrepo",
	}
	for k, v := range data {
		assert.True(t, SameURL(k, v))
	}
}

func TestNormalizeURL(t *testing.T) {
	testData := []struct {
		normalizedUrl string
		url           string
	}{
		{normalizedUrl: "https://github.com/argoproj/test", url: "https://github.com/argoproj/test.git"},
		{normalizedUrl: "git@github.com:argoproj/test", url: "ssh://git@github.com:argoproj/test"},
	}
	for _, data := range testData {
		t.Run(data.url, func(t *testing.T) {
			assert.Equal(t, data.normalizedUrl, NormalizeURL(data.url))
		})
	}
}
