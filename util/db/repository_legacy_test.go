package db

import (
	"testing"

	lru "github.com/hashicorp/golang-lru"

	"github.com/argoproj/argo-cd/v2/util/settings"
)

func Test_getRepositoryCredentialIndex(t *testing.T) {
	cache, _ := lru.New(100)
	repositoryCredentials := []settings.RepositoryCredentials{
		{URL: "http://known"},
		{URL: "http://known/repos"},
		{URL: "http://known/other"},
		{URL: "http://known/other/other"},
	}
	tests := []struct {
		name    string
		repoURL string
		want    int
	}{
		{"TestNotFound", "", -1},
		{"TestNotFound", "http://unknown/repos", -1},
		{"TestNotFound", "http://unknown/repo/repo", -1},
		{"TestFoundFound", "http://known/repos/repo", 1},
		{"TestFoundFound", "http://known/other/repo/foo", 2},
		{"TestFoundFound", "http://known/other/other/repo", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getRepositoryCredentialIndex(cache, repositoryCredentials, tt.repoURL); got != tt.want {
				t.Errorf("getRepositoryCredentialIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}
