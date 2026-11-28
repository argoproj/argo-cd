package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func helmRepoSecret(secretName string, data map[string]string) *corev1.Secret {
	secretData := map[string][]byte{}
	for k, v := range data {
		secretData[k] = []byte(v)
	}
	return &corev1.Secret{
		Namespace: testNamespace,
		Name:      secretName,
		Labels: map[string]string{
			common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
		},
		Data: secretData,
	}
}

func TestListHelmRepositories(t *testing.T) {
	clientset := getClientset(
		helmRepoSecret("named-helm", map[string]string{"type": "helm", "name": "named", "url": "https://charts.example.com"}),
		// A classic Helm repository without a name cannot be added with `helm repo add`, so it is skipped.
		helmRepoSecret("nameless-helm", map[string]string{"type": "helm", "url": "https://nameless.example.com"}),
		// An OCI registry is logged in to by hostname and does not need a name.
		helmRepoSecret("nameless-oci", map[string]string{"type": "helm", "enableOCI": "true", "url": "registry.example.com/charts", "username": "user", "password": "pass"}),
		helmRepoSecret("git", map[string]string{"type": "git", "name": "git", "url": "https://github.com/argoproj/argocd-example-apps"}),
	)
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

	repos, err := db.ListHelmRepositories(t.Context())
	require.NoError(t, err)

	urls := make([]string, 0, len(repos))
	for _, repo := range repos {
		urls = append(urls, repo.Repo)
	}
	assert.ElementsMatch(t, []string{"https://charts.example.com", "registry.example.com/charts"}, urls)
}
