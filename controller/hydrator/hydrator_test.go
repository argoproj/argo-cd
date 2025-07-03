package hydrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	mockcommitclient "github.com/argoproj/argo-cd/v3/commitserver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	reposerver "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	mockrepoclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v3/util/argo"
	log "github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/controller/hydrator/mocks"
	"github.com/argoproj/argo-cd/v3/controller/hydrator/types"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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

type MockHydratorDependencies struct {
	mock.Mock
}

func (m *MockHydratorDependencies) LogHydrationPhaseEvent(ctx context.Context, app *appv1.Application, eventInfo argo.EventInfo, message string) {
	m.Called(ctx, app, eventInfo, message)
}

func (m *MockHydratorDependencies) PersistAppHydratorStatus(origApp *appv1.Application, status *appv1.SourceHydratorStatus) {
	m.Called(origApp, status)
}

func (m *MockHydratorDependencies) AddHydrationQueueItem(key HydrationQueueKey) {
	m.Called(key)
}

func (m *MockHydratorDependencies) GetProcessableAppProj(app *appv1.Application) (*appv1.AppProject, error) {
	args := m.Called(app)
	return args.Get(0).(*appv1.AppProject), args.Error(1)
}

func (m *MockHydratorDependencies) GetProcessableApps() (*appv1.ApplicationList, error) {
	args := m.Called()
	return args.Get(0).(*appv1.ApplicationList), args.Error(1)
}

func (m *MockHydratorDependencies) GetRepoObjs(app *appv1.Application, source appv1.ApplicationSource, revision string, project *appv1.AppProject) ([]*unstructured.Unstructured, *reposerver.ManifestResponse, error) {
	args := m.Called(app, source, revision, project)
	return args.Get(0).([]*unstructured.Unstructured), args.Get(1).(*reposerver.ManifestResponse), args.Error(2)
}

func (m *MockHydratorDependencies) GetWriteCredentials(ctx context.Context, repoURL string, project string) (*appv1.Repository, error) {
	args := m.Called(ctx, repoURL, project)
	return args.Get(0).(*appv1.Repository), args.Error(1)
}

func (m *MockHydratorDependencies) RequestAppRefresh(appName string, appNamespace string) error {
	args := m.Called(appName, appNamespace)
	return args.Error(0)
}

func TestLogsHydrationEvent_HydrationStarted(t *testing.T) {

	now := metav1.NewTime(time.Now())
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))

	app := &v1alpha1.Application{
		Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
		Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{StartedAt: oneHourAgo}}},
	}
	// Create mock dependencies
	mockDeps := &MockHydratorDependencies{}

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

	mockDeps.On("PersistAppHydratorStatus", mock.Anything, mock.Anything).Once()
	mockDeps.On("AddHydrationQueueItem", mock.Anything).Once()

	hydrator.ProcessAppHydrateQueueItem(app)

	// Verify all expectations were met
	mockDeps.AssertExpectations(t)

	// Additional assertions on app state
	// assert.Equal(t, appv1.HydrateOperationPhaseHydrating, app.Status.SourceHydrator.CurrentOperation.Phase)
	assert.NotNil(t, app.Status.SourceHydrator.CurrentOperation.StartedAt)
}

type MockRepoGetter struct {
	mock.Mock
}

func (m *MockRepoGetter) GetRepository(ctx context.Context, repoURL string, project string) (*v1alpha1.Repository, error) {
	args := m.Called(ctx, repoURL, project)
	return args.Get(0).(*v1alpha1.Repository), args.Error(1)
}

type MockCloser struct {
	mock.Mock
}

func (m *MockCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestLogsHydrationEvent_HydrationCompleted(t *testing.T) {

	now := metav1.NewTime(time.Now())
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))

	log.SetLevel(log.TraceLevel)

	app := &v1alpha1.Application{
		Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{DrySource: v1alpha1.DrySource{RepoURL: "https://example.com/repo.git", TargetRevision: "main"}, SyncSource: v1alpha1.SyncSource{Path: "dev", TargetBranch: "dev"}}},
		Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{StartedAt: oneHourAgo}}},
	}
	// Create mock dependencies
	mockDeps := &MockHydratorDependencies{}
	// create commitServer mock	clientset
	mockCommitClient := mockcommitclient.CommitServiceClient{}
	commitServerMockClientset := mockcommitclient.NewClientset(t)
	// repoServer mocks
	mockRepoClient := mockrepoclient.RepoServerServiceClient{}
	mockRepoClientset := mockrepoclient.Clientset{RepoServerServiceClient: &mockRepoClient}
	// Create a mock repoGetter
	repoGetter := &MockRepoGetter{}
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
	mockDeps.On("GetProcessableApps").Return(&appv1.ApplicationList{
		Items: []appv1.Application{
			*app,
		},
	}, nil)
	mockDeps.On("GetProcessableAppProj", mock.AnythingOfType("*v1alpha1.Application")).Return(&appv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "default",
			Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		},
		Spec: appv1.AppProjectSpec{
			SourceRepos: []string{"https://example.com/repo.git"},
			Destinations: []appv1.ApplicationDestination{
				{
					Name:      "default",
					Namespace: "default",
				},
			},
		},
	}, nil)
	mockDeps.On("GetRepoObjs", mock.AnythingOfType("*v1alpha1.Application"), mock.AnythingOfType("v1alpha1.ApplicationSource"), mock.AnythingOfType("string"), mock.AnythingOfType("*v1alpha1.AppProject")).Return([]*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "example-configmap",
					"namespace": "default",
				},
				"data": map[string]interface{}{
					"key": "value",
				},
			},
		},
	}, &reposerver.ManifestResponse{}, nil).Once()
	// define expected response for commitHydratedManifests
	mockCommitClient.EXPECT().CommitHydratedManifests(mock.Anything, mock.Anything).Return(&apiclient.CommitHydratedManifestsResponse{HydratedSha: "hydrated-sha"}, error(nil)).Once()
	mockCloser := &MockCloser{}
	mockCloser.On("Close").Return(nil)
	commitServerMockClientset.EXPECT().NewCommitServerClient().Return(mockCloser, &mockCommitClient, nil).Once()
	// define expected response for GetRepository
	repoGetter.On("GetRepository", mock.Anything, "https://example.com/repo.git", "default").Return(&appv1.Repository{
		Repo: "https://example.com/repo.git",
	}, nil).Once()
	// define expected response for GetRevisionMetadata
	mockRepoClient.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(&appv1.RevisionMetadata{
		Message: "some-message",
	}, nil).Once()
	mockDeps.On("GetWriteCredentials", mock.Anything, "https://example.com/repo.git", "default").Return(&appv1.Repository{
		Repo: "https://example.com/repo.git",
	}, nil).Once()
	mockDeps.On("RequestAppRefresh", app.Name, app.Namespace).Return(nil).Once()
	hydrator.ProcessAppHydrateQueueItem(app)
	hydrator.ProcessHydrationQueueItem(HydrationQueueKey{
		SourceRepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
		SourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		DestinationBranch:    app.Spec.SourceHydrator.SyncSource.TargetBranch,
	})
	// // Verify all expectations were met
	mockDeps.AssertExpectations(t)
}
func TestLogsHydrationEvent_HydrationFailed(t *testing.T) {

	now := metav1.NewTime(time.Now())
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))

	log.SetLevel(log.TraceLevel)

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
	mockDeps := &MockHydratorDependencies{}
	// create commitServer mock	clientset
	commitServerMockClientset := mockcommitclient.NewClientset(t)
	// repoServer mocks
	mockRepoClient := mockrepoclient.RepoServerServiceClient{}
	mockRepoClientset := mockrepoclient.Clientset{RepoServerServiceClient: &mockRepoClient}
	// Create a mock repoGetter
	repoGetter := &MockRepoGetter{}
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
	mockDeps.On("PersistAppHydratorStatus", mock.Anything, mock.Anything).Twice()
	mockDeps.On("AddHydrationQueueItem", mock.Anything, mock.Anything).Once()
	mockDeps.On("GetProcessableApps").Return(&appv1.ApplicationList{
		Items: []appv1.Application{
			*app,
		},
	}, nil)
	mockDeps.On("GetProcessableAppProj", mock.AnythingOfType("*v1alpha1.Application")).Return(&appv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "default",
			Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		},
		Spec: appv1.AppProjectSpec{
			SourceRepos: []string{"https://example.com/repo.git"},
			Destinations: []appv1.ApplicationDestination{
				{
					Name:      "default",
					Namespace: "default",
				},
			},
		},
	}, nil)
	mockDeps.On("GetRepoObjs", mock.AnythingOfType("*v1alpha1.Application"), mock.AnythingOfType("v1alpha1.ApplicationSource"), mock.AnythingOfType("string"), mock.AnythingOfType("*v1alpha1.AppProject")).Return([]*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "example-configmap",
					"namespace": "default",
				},
				"data": map[string]interface{}{
					"key": "value",
				},
			},
		},
	}, &reposerver.ManifestResponse{}, errors.New("Obj error")).Once()
	hydrator.ProcessAppHydrateQueueItem(app)
	hydrator.ProcessHydrationQueueItem(HydrationQueueKey{
		SourceRepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
		SourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
		DestinationBranch:    app.Spec.SourceHydrator.SyncSource.TargetBranch,
	})
	// // Verify all expectations were met
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
