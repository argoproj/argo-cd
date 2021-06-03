package db

import (
	"testing"
)

func TestRepoURLToSecretName(t *testing.T) {
	tables := map[string]string{
		"git://git@github.com:argoproj/ARGO-cd.git": "repo-83273445",
		"https://github.com/argoproj/ARGO-cd":       "repo-1890113693",
		"https://github.com/argoproj/argo-cd":       "repo-42374749",
		"https://github.com/argoproj/argo-cd.git":   "repo-821842295",
		"https://github.com/argoproj/argo_cd.git":   "repo-1049844989",
		"ssh://git@github.com/argoproj/argo-cd.git": "repo-3569564120",
	}

	for k, v := range tables {
		if sn := RepoURLToSecretName(repoSecretPrefix, k); sn != v {
			t.Errorf("Expected secret name %q for repo %q; instead, got %q", v, k, sn)
		}
	}
}

func Test_CredsURLToSecretName(t *testing.T) {
	tables := map[string]string{
		"git://git@github.com:argoproj":  "creds-2483499391",
		"git://git@github.com:argoproj/": "creds-1465032944",
		"git@github.com:argoproj":        "creds-2666065091",
		"git@github.com:argoproj/":       "creds-346879876",
	}

	for k, v := range tables {
		if sn := RepoURLToSecretName(credSecretPrefix, k); sn != v {
			t.Errorf("Expected secret name %q for repo %q; instead, got %q", v, k, sn)
		}
	}
}
