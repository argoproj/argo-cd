package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSSHUrl(t *testing.T) {
	data := map[string]bool{
		"git@GITHUB.com:argoproj/test.git":     true,
		"https://github.com/argoproj/test.git": false,
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
