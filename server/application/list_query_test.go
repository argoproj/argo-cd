package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func ptr[T any](v T) *T { return &v }

func TestMatchesApplicationQuerySearch(t *testing.T) {
	app := &v1alpha1.Application{
		Name: "guestbook", Namespace: "argocd",
		Spec: v1alpha1.ApplicationSpec{Project: "default"},
		Status: v1alpha1.ApplicationStatus{
			Sync:   v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeSynced},
			Health: v1alpha1.AppHealthStatus{Status: "Healthy"},
		},
	}

	assert.True(t, matchesApplicationQueryQ(app, &application.ApplicationQuery{Search: ptr("guest")}))
	assert.False(t, matchesApplicationQueryQ(app, &application.ApplicationQuery{Search: ptr("other")}))
	assert.True(t, matchesApplicationQueryQ(app, &application.ApplicationQuery{Search: ptr("^guest"), SearchRegex: ptr(true)}))
	assert.False(t, matchesApplicationQueryQ(app, &application.ApplicationQuery{Search: ptr("["), SearchRegex: ptr(true)}))
	assert.False(t, matchesApplicationQueryQ(app, &application.ApplicationQuery{SyncStatuses: []string{"OutOfSync"}}))
	assert.True(t, matchesApplicationQueryQ(app, &application.ApplicationQuery{SyncStatuses: []string{"Synced"}}))
}

func TestPaginateApplications(t *testing.T) {
	apps := []*v1alpha1.Application{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}
	require.Len(t, paginateApplications(apps, &application.ApplicationQuery{}), 3)
	page := paginateApplications(apps, &application.ApplicationQuery{Offset: ptr(int64(1)), Limit: ptr(int64(1))})
	require.Len(t, page, 1)
	assert.Equal(t, "b", page[0].Name)
}

func TestBuildApplicationListStats(t *testing.T) {
	apps := []*v1alpha1.Application{
		{
			Name: "a", Labels: map[string]string{"env": "prod"},
			Spec: v1alpha1.ApplicationSpec{
				Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/a"},
				Destination: v1alpha1.ApplicationDestination{Server: "https://k8s", Namespace: "ns1"},
			},
			Status: v1alpha1.ApplicationStatus{Sync: v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeSynced}, Health: v1alpha1.AppHealthStatus{Status: "Healthy"}},
		},
		{
			Name: "b",
			Spec: v1alpha1.ApplicationSpec{
				Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/b"},
				Destination: v1alpha1.ApplicationDestination{Server: "https://k8s", Namespace: "ns2"},
			},
			Status: v1alpha1.ApplicationStatus{Sync: v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeOutOfSync}, Health: v1alpha1.AppHealthStatus{Status: "Degraded"}},
		},
	}
	stats := buildApplicationListStats(apps, &application.ApplicationQuery{})
	assert.Equal(t, int64(2), stats.Total)
	assert.Equal(t, int64(1), stats.TotalBySyncStatus[v1alpha1.SyncStatusCodeSynced])
	assert.Equal(t, int64(1), stats.TotalBySyncStatus[v1alpha1.SyncStatusCodeOutOfSync])
	assert.Equal(t, []string{"https://github.com/example/a", "https://github.com/example/b"}, stats.Repos)
}
