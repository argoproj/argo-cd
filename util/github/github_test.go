package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOwnerAndRepoNameExtraction(t *testing.T) {
	data := [][]string{
		{"git@GITHUB.com:argoproj/test", "argoproj", "test"},
		{"git@GITHUB.com:argoproj/test.git", "argoproj", "test"},
		{"https://GITHUB.com/argoproj/test", "argoproj", "test"},
		{"https://GITHUB.com/argoproj/test.git", "argoproj", "test"},
		{"https://GITHUB.com/argoproj", "", ""},
		{"https://github.com/TEST", "", ""},
		{"https://github.com:4443/TEST", "", ""},
		{"https://github.com:4443/TEST.git", "", ""},
		{"ssh://git@GITHUB.com/argoproj/test", "argoproj", "test"},
		{"ssh://git@GITHUB.com/argoproj/test.git", "argoproj", "test"},
		{"ssh://git@GITHUB.com/test", "", ""},
	}
	for _, table := range data {
		owner, repo := OwnerAndRepoName(table[0])
		assert.Equal(t, table[1], owner)
		assert.Equal(t, table[2], repo)
	}
}
