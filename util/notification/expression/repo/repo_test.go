package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFullNameByRepoURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/argoproj/argo-cd.git", "argoproj/argo-cd"},
		{"git@github.com:argoproj/argo-cd.git", "argoproj/argo-cd"},
		{"https://gitlab.com/group/project.git", "group/project"},
		{"https://gitlab.example.com/group/sub/subsub/project.git", "group/sub/subsub/project"},
		{"git@gitlab.example.com:group/sub/project.git", "group/sub/project"},
		{"ssh://git@gitlab.example.com:2222/group/sub/project.git", "group/sub/project"},
		{"https://bitbucket.org/workspace/repo.git", "workspace/repo"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.expected, FullNameByRepoURL(tt.url))
		})
	}
}
