package db

import "testing"

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
