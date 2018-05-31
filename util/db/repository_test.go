package db

import "testing"

func TestRepoURLToSecretName(t *testing.T) {
	tables := map[string]string{
		"git://git@github.com:argoproj/ARGO-cd.git": "repo-argo-cd-593837413",
		"https://github.com/argoproj/ARGO-cd":       "repo-argo-cd-821842295",
		"https://github.com/argoproj/argo-cd":       "repo-argo-cd-821842295",
		"https://github.com/argoproj/argo-cd.git":   "repo-argo-cd-821842295",
		"ssh://git@github.com/argoproj/argo-cd.git": "repo-argo-cd-1019298066",
	}

	for k, v := range tables {
		if sn := RepoURLToSecretName(k); sn != v {
			t.Errorf("Expected secret name %q for repo %q; instead, got %q", v, k, sn)
		}
	}
}
