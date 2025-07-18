package hydrator

import (
	"errors"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	commitclient "github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	mockcommitclient "github.com/argoproj/argo-cd/v3/commitserver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	reposerver "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	mockrepoclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v3/util/argo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/controller/hydrator/mocks"
	"github.com/argoproj/argo-cd/v3/controller/hydrator/types"
)

func Test_appNeedsHydration(t *testing.T) {
	t.Parallel()

	now := metav1.NewTime(time.Now())
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))

	testCases := []struct {
		name                   string
		app                    *v1alpha1.Application
		timeout                time.Duration
		expectedNeedsHydration bool
		expectedMessage        string
	}{
		{
			name:                   "source hydrator not configured",
			app:                    &v1alpha1.Application{},
			expectedNeedsHydration: false,
			expectedMessage:        "source hydrator not configured",
		},
		{
			name: "hydrate requested",
			app: &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1alpha1.AnnotationKeyHydrate: "normal"}},
				Spec:       v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
			},
			timeout:                1 * time.Hour,
			expectedNeedsHydration: true,
			expectedMessage:        "hydrate requested",
		},
		{
			name: "no previous hydrate operation",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
			},
			timeout:                1 * time.Hour,
			expectedNeedsHydration: true,
			expectedMessage:        "no previous hydrate operation",
		},
		{
			name: "spec.sourceHydrator differs",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
				Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{
					SourceHydrator: v1alpha1.SourceHydrator{DrySource: v1alpha1.DrySource{RepoURL: "something new"}},
				}}},
			},
			timeout:                1 * time.Hour,
			expectedNeedsHydration: true,
			expectedMessage:        "spec.sourceHydrator differs",
		},
		{
			name: "hydration failed more than two minutes ago",
			app: &v1alpha1.Application{
				Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
				Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{DrySHA: "abc123", FinishedAt: &oneHourAgo, Phase: v1alpha1.HydrateOperationPhaseFailed}}},
			},
			timeout:                1 * time.Hour,
			expectedNeedsHydration: true,
			expectedMessage:        "previous hydrate operation failed more than 2 minutes ago",
		},
		{
			name: "timeout reached",
			app: &v1alpha1.Application{
				Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
				Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{StartedAt: oneHourAgo}}},
			},
			timeout:                1 * time.Minute,
			expectedNeedsHydration: true,
			expectedMessage:        "hydration expired",
		},
		{
			name: "hydrate not needed",
			app: &v1alpha1.Application{
				Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
				Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{DrySHA: "abc123", StartedAt: now, FinishedAt: &now, Phase: v1alpha1.HydrateOperationPhaseFailed}}},
			},
			timeout:                1 * time.Hour,
			expectedNeedsHydration: false,
			expectedMessage:        "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			needsHydration, message := appNeedsHydration(tc.app, tc.timeout)
			assert.Equal(t, tc.expectedNeedsHydration, needsHydration)
			assert.Equal(t, tc.expectedMessage, message)
		})
	}
}

func TestLogsHydrationEvent_HydrationStarted(t *testing.T) {
	now := metav1.NewTime(time.Now())
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))

	app := &v1alpha1.Application{
		Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
		Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{StartedAt: oneHourAgo}}},
	}
	// Create mock dependencies
	mockDeps := mocks.NewDependencies(t)

	hydrator := &Hydrator{
		dependencies:         mockDeps,
		statusRefreshTimeout: time.Minute * 5,
		commitClientset:      mockcommitclient.NewClientset(t),
	}
	// Set up mock expectations
	mockDeps.On("LogHydrationPhaseEvent",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Application"),
		mock.MatchedBy(func(eventInfo argo.EventInfo) bool {
			return eventInfo.Reason == argo.EventReasonHydrationStarted && eventInfo.Type == corev1.EventTypeNormal
		}),
		"Hydration started").Once()

	mockDeps.On("PersistAppHydratorStatus", mock.Anything, mock.Anything)
	mockDeps.On("AddHydrationQueueItem", mock.Anything)

	hydrator.ProcessAppHydrateQueueItem(app)

	// Verify all expectations were met
	mockDeps.AssertExpectations(t)

	// Additional assertions on app state
	// assert.Equal(t, v1alpha1.HydrateOperationPhaseHydrating, app.Status.SourceHydrator.CurrentOperation.Phase)
	assert.NotNil(t, app.Status.SourceHydrator.CurrentOperation.StartedAt)
}

type MockCloser struct {
	mock.Mock
}

func (m *MockCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

func Test_LogsHydrationEvent_HydrationCompleted(t *testing.T) {
	now := metav1.NewTime(time.Now())
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))

	log.SetLevel(log.TraceLevel)

	app := &v1alpha1.Application{
		Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{DrySource: v1alpha1.DrySource{RepoURL: "https://example.com/repo.git", TargetRevision: "main"}, SyncSource: v1alpha1.SyncSource{Path: "dev", TargetBranch: "dev"}}},
		Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{StartedAt: oneHourAgo}}},
	}
	// Create mock dependencies
	mockDeps := mocks.NewDependencies(t)
	// create commitServer mock	clientset
	mockCommitClient := mockcommitclient.CommitServiceClient{}
	commitServerMockClientset := mockcommitclient.NewClientset(t)
	// repoServer mocks
	mockRepoClient := mockrepoclient.RepoServerServiceClient{}
	mockRepoClientset := mockrepoclient.Clientset{RepoServerServiceClient: &mockRepoClient}
	// Create a mock repoGetter
	repoGetter := mocks.NewRepoGetter(t)
	// create Hydrator instance
	hydrator := &Hydrator{
		dependencies:         mockDeps,
		statusRefreshTimeout: time.Minute * 5,
		commitClientset:      commitServerMockClientset,
		repoClientset:        &mockRepoClientset,
		repoGetter:           repoGetter,
	}
	// Set up mock expectations
	mockDeps.On("LogHydrationPhaseEvent",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Application"),
		mock.MatchedBy(func(eventInfo argo.EventInfo) bool {
			return eventInfo.Reason == argo.EventReasonHydrationStarted && eventInfo.Type == corev1.EventTypeNormal
		}),
		"Hydration started").Once()

	mockDeps.On("LogHydrationPhaseEvent",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Application"),
		mock.MatchedBy(func(eventInfo argo.EventInfo) bool {
			return eventInfo.Reason == argo.EventReasonHydrationCompleted && eventInfo.Type == corev1.EventTypeNormal
		}),
		"Hydration completed").Once()

	mockDeps.On("PersistAppHydratorStatus", mock.Anything, mock.Anything).Twice()
	mockDeps.On("AddHydrationQueueItem", mock.Anything, mock.Anything).Once()
	mockDeps.On("GetProcessableApps").Return(&v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{
			*app,
		},
	}, nil)

	mockDeps.On("GetProcessableAppProj", mock.AnythingOfType("*v1alpha1.Application")).Return(&v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "default",
			Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"https://example.com/repo.git"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Name:      "default",
					Namespace: "default",
				},
			},
		},
	}, nil)

	mockDeps.On("GetRepoObjs", mock.AnythingOfType("*v1alpha1.Application"), mock.AnythingOfType("v1alpha1.ApplicationSource"), mock.AnythingOfType("string"), mock.AnythingOfType("*v1alpha1.AppProject")).Return([]*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "example-configmap",
					"namespace": "default",
				},
				"data": map[string]any{
					"key": "value",
				},
			},
		},
	}, &reposerver.ManifestResponse{}, nil)

	// define expected response for commitHydratedManifests
	mockCommitClient.EXPECT().CommitHydratedManifests(mock.Anything, mock.Anything).Return(&commitclient.CommitHydratedManifestsResponse{HydratedSha: "hydrated-sha"}, error(nil))
	mockCloser := &MockCloser{}
	mockCloser.On("Close").Return(nil)
	commitServerMockClientset.EXPECT().NewCommitServerClient().Return(mockCloser, &mockCommitClient, nil)
	// define expected response for GetRepository
	repoGetter.On("GetRepository", mock.Anything, "https://example.com/repo.git", "default").Return(&v1alpha1.Repository{Repo: "https://example.com/repo.git"}, nil)
	// define expected response for GetRevisionMetadata
	mockRepoClient.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(&v1alpha1.RevisionMetadata{Message: "some-message"}, nil)
	mockDeps.On("GetWriteCredentials", mock.Anything, "https://example.com/repo.git", "default").Return(&v1alpha1.Repository{Repo: "https://example.com/repo.git"}, nil)
	mockDeps.On("RequestAppRefresh", app.Name, app.Namespace).Return(nil)
	// Run the hydrator process
	hydrator.ProcessAppHydrateQueueItem(app)
	hydrator.ProcessHydrationQueueItem(types.HydrationQueueKey{
		SourceRepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
		SourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		DestinationBranch:    app.Spec.SourceHydrator.SyncSource.TargetBranch,
	})

	// Verify all expectations were met
	mockDeps.AssertExpectations(t)
}

func Test_LogsHydrationEvent_HydrationFailed(t *testing.T) {
	now := metav1.NewTime(time.Now())
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))

	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{v1alpha1.AnnotationKeyHydrate: "normal"},
			Name:        "test-app",
			Namespace:   "default",
		},
		Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{DrySource: v1alpha1.DrySource{RepoURL: "https://example.com/repo.git", TargetRevision: "main"}, SyncSource: v1alpha1.SyncSource{Path: "dev", TargetBranch: "dev"}}},
		Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{StartedAt: oneHourAgo}}},
	}
	// Create mock dependencies
	mockDeps := mocks.NewDependencies(t)
	// create commitServer mock	clientset
	commitServerMockClientset := mockcommitclient.NewClientset(t)
	// repoServer mocks
	mockRepoClient := mockrepoclient.RepoServerServiceClient{}
	mockRepoClientset := mockrepoclient.Clientset{RepoServerServiceClient: &mockRepoClient}
	// Create a mock repoGetter
	repoGetter := mocks.NewRepoGetter(t)
	// create Hydrator instance
	hydrator := &Hydrator{
		dependencies:         mockDeps,
		statusRefreshTimeout: time.Minute * 5,
		commitClientset:      commitServerMockClientset,
		repoClientset:        &mockRepoClientset,
		repoGetter:           repoGetter,
	}
	// Set up mock expectations
	mockDeps.On("LogHydrationPhaseEvent",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Application"),
		mock.MatchedBy(func(eventInfo argo.EventInfo) bool {
			return eventInfo.Reason == argo.EventReasonHydrationStarted && eventInfo.Type == corev1.EventTypeNormal
		}),
		"Hydration started").Once()
	mockDeps.On("LogHydrationPhaseEvent",
		mock.Anything,
		mock.AnythingOfType("*v1alpha1.Application"),
		mock.MatchedBy(func(eventInfo argo.EventInfo) bool {
			return eventInfo.Reason == argo.EventReasonHydrationFailed && eventInfo.Type == corev1.EventTypeWarning
		}),
		mock.MatchedBy(func(message string) bool {
			return strings.Contains(message, "Failed to hydrate app")
		})).Once()
	mockDeps.On("PersistAppHydratorStatus", mock.Anything, mock.Anything)
	mockDeps.On("AddHydrationQueueItem", mock.Anything, mock.Anything)
	mockDeps.On("GetProcessableApps").Return(&v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{
			*app,
		},
	}, nil)
	mockDeps.On("GetProcessableAppProj", mock.AnythingOfType("*v1alpha1.Application")).Return(&v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "default",
			Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"https://example.com/repo.git"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Name:      "default",
					Namespace: "default",
				},
			},
		},
	}, nil)
	mockDeps.On("GetRepoObjs", mock.AnythingOfType("*v1alpha1.Application"), mock.AnythingOfType("v1alpha1.ApplicationSource"), mock.AnythingOfType("string"), mock.AnythingOfType("*v1alpha1.AppProject")).Return([]*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "example-configmap",
					"namespace": "default",
				},
				"data": map[string]any{
					"key": "value",
				},
			},
		},
	}, &reposerver.ManifestResponse{}, errors.New("Obj error")).Once()

	// Run the hydrator process
	hydrator.ProcessAppHydrateQueueItem(app)
	hydrator.ProcessHydrationQueueItem(types.HydrationQueueKey{
		SourceRepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
		SourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		DestinationBranch:    app.Spec.SourceHydrator.SyncSource.TargetBranch,
	})

	// Verify all expectations were met
	mockDeps.AssertExpectations(t)
}

func Test_getRelevantAppsForHydration_RepoURLNormalization(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	d.On("GetProcessableApps").Return(&v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{
			{
				Spec: v1alpha1.ApplicationSpec{
					Project: "project",
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        "https://example.com/repo.git",
							TargetRevision: "main",
							Path:           "app1",
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "main",
							Path:         "app1",
						},
					},
				},
			},
			{
				Spec: v1alpha1.ApplicationSpec{
					Project: "project",
					SourceHydrator: &v1alpha1.SourceHydrator{
						DrySource: v1alpha1.DrySource{
							RepoURL:        "https://example.com/repo",
							TargetRevision: "main",
							Path:           "app2",
						},
						SyncSource: v1alpha1.SyncSource{
							TargetBranch: "main",
							Path:         "app2",
						},
					},
				},
			},
		},
	}, nil)
	d.On("GetProcessableAppProj", mock.Anything).Return(&v1alpha1.AppProject{
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"https://example.com/*"},
		},
	}, nil)

	hydrator := &Hydrator{dependencies: d}

	hydrationKey := types.HydrationQueueKey{
		SourceRepoURL:        "https://example.com/repo",
		SourceTargetRevision: "main",
		DestinationBranch:    "main",
	}

	logCtx := log.WithField("test", "RepoURLNormalization")
	relevantApps, err := hydrator.getRelevantAppsForHydration(logCtx, hydrationKey)

	require.NoError(t, err)
	assert.Len(t, relevantApps, 2, "Expected both apps to be considered relevant despite URL differences")
}
