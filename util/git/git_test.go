package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReturnsTrueForSSHUrl(t *testing.T) {
	assert.True(t, IsSshURL("git@github.com:test.git"))
}

func TestReturnsFalseForNonSSHUrl(t *testing.T) {
	assert.False(t, IsSshURL("https://github.com/test.git"))
}

func TestNormalizeUrl(t *testing.T) {
	assert.Equal(t, NormalizeGitURL("git@GITHUB.com:test.git"), "git@github.com:test.git")
}
