package argo

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func testApp(name, ns string) *argoappv1.Application {
	return &argoappv1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: argoappv1.ApplicationSpec{
			Project: "default",
			Source:  &argoappv1.ApplicationSource{RepoURL: "https://example.com/repo.git"},
			Destination: argoappv1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "app-ns",
			},
		},
		Status: argoappv1.ApplicationStatus{
			Sync:   argoappv1.SyncStatus{Status: argoappv1.SyncStatusCodeSynced},
			Health: argoappv1.AppHealthStatus{Status: "Healthy"},
		},
	}
}

func TestMatchesSearch(t *testing.T) {
	app := testApp("my-app", "argocd")

	assert.True(t, MatchesSearch(app, "", false))
	assert.True(t, MatchesSearch(app, "my-", false))
	assert.True(t, MatchesSearch(app, "argocd", false))
	assert.False(t, MatchesSearch(app, "other", false))
	assert.True(t, MatchesSearch(app, "^my-.*$", true))
	assert.False(t, MatchesSearch(app, "[", true))

	re := mustCompile(`^my-`)
	assert.True(t, MatchesSearchCompiled(app, "", re))
	assert.False(t, MatchesSearchCompiled(testApp("other", "argocd"), "", re))
	assert.True(t, MatchesSearchCompiled(app, "my-", nil))
}

func mustCompile(expr string) *regexp.Regexp {
	re, err := regexp.Compile(expr)
	if err != nil {
		panic(err)
	}
	return re
}

func TestMatchesReposAndClusters(t *testing.T) {
	app := testApp("my-app", "argocd")
	assert.True(t, MatchesRepos(app, nil))
	assert.True(t, MatchesRepos(app, []string{"https://example.com/repo.git"}))
	assert.False(t, MatchesRepos(app, []string{"https://other"}))

	assert.True(t, MatchesClusters(app, []string{"https://kubernetes.default.svc"}))
	assert.False(t, MatchesClusters(app, []string{"https://other"}))
	assert.True(t, MatchesNamespaces(app, []string{"app-ns"}))
}

func TestMatchesSyncHealthAutoSync(t *testing.T) {
	app := testApp("my-app", "argocd")
	assert.True(t, MatchesSyncStatuses(app, []string{"Synced"}))
	assert.False(t, MatchesSyncStatuses(app, []string{"OutOfSync"}))
	assert.True(t, MatchesHealthStatuses(app, []string{"Healthy"}))
	assert.True(t, MatchesAutoSync(app, []string{"Disabled"}))
	app.Spec.SyncPolicy = &argoappv1.SyncPolicy{Automated: &argoappv1.SyncPolicyAutomated{}}
	assert.True(t, MatchesAutoSync(app, []string{"Enabled"}))
}
