package db

import (
	"testing"

	"github.com/argoproj/argo-cd/util/settings"
)

func TestRepoURLToSecretName(t *testing.T) {
	tables := map[string]string{
		"git://git@github.com:argoproj/ARGO-cd.git": "repo-argo-cd-83273445",
		"https://github.com/argoproj/ARGO-cd":       "repo-argo-cd-1890113693",
		"https://github.com/argoproj/argo-cd":       "repo-argo-cd-42374749",
		"https://github.com/argoproj/argo-cd.git":   "repo-argo-cd-821842295",
		"https://github.com/argoproj/argo_cd.git":   "repo-argo-cd-1049844989",
		"ssh://git@github.com/argoproj/argo-cd.git": "repo-argo-cd-3569564120",
	}

	for k, v := range tables {
		if sn := repoURLToSecretName(k); sn != v {
			t.Errorf("Expected secret name %q for repo %q; instead, got %q", v, k, sn)
		}
	}
}

func Test_getRepositoryCredentialIndex(t *testing.T) {
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
			if got := getRepositoryCredentialIndex(repositoryCredentials, tt.repoURL); got != tt.want {
				t.Errorf("getRepositoryCredentialIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}
