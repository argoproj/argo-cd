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

type MockCloser struct {
	mock.Mock
}

func (m *MockCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

// setupCommonMocks creates a hydrator instance with common mocked dependencies
func setupCommonMocks(t *testing.T, overrides map[string]interface{}) (*Hydrator, *mocks.Dependencies, *mockcommitclient.Clientset, *mockrepoclient.Clientset, *mocks.RepoGetter) {
	mockDeps := mocks.NewDependencies(t)

	// Create commit server mock clientset
	commitServerMockClientset := mockcommitclient.NewClientset(t)

	// Create repo server mocks
	mockRepoClient := mockrepoclient.RepoServerServiceClient{}
	mockRepoClientset := &mockrepoclient.Clientset{RepoServerServiceClient: &mockRepoClient}

	// Create a mock repoGetter
	repoGetter := mocks.NewRepoGetter(t)

	// Create hydrator instance
	hydrator := &Hydrator{
		dependencies:         mockDeps,
		statusRefreshTimeout: time.Minute * 5,
		commitClientset:      commitServerMockClientset,
		repoClientset:        mockRepoClientset,
		repoGetter:           repoGetter,
	}
	// Set up default expectations for the hydrator methods
	mockDeps.On("AddHydrationQueueItem", mock.Anything, mock.Anything)

	// Apply any overrides from the test
	if overrides != nil {
		for method, behavior := range overrides {
			switch method {
			case "LogHydrationPhaseEvent":
				if behavior != nil {
					mockDeps.On("LogHydrationPhaseEvent", behavior.([]interface{})...)
				}
			case "GetProcessableApps":
				if behavior != nil {
					mockDeps.On("GetProcessableApps").Return(behavior.([]interface{})...)
				}
			case "GetProcessableAppProj":
				if behavior != nil {
					mockDeps.On("GetProcessableAppProj", mock.AnythingOfType("*v1alpha1.Application")).Return(behavior.([]interface{})...)
				}
			case "GetRepoObjs":
				if behavior != nil {
					mockDeps.On("GetRepoObjs", mock.AnythingOfType("*v1alpha1.Application"), mock.AnythingOfType("v1alpha1.ApplicationSource"), mock.AnythingOfType("string"), mock.AnythingOfType("*v1alpha1.AppProject")).Return(behavior.([]interface{})...)
				}
			case "GetWriteCredentials":
				if behavior != nil {
					mockDeps.On("GetWriteCredentials", mock.Anything, "https://example.com/repo.git", "default").Return(behavior.([]interface{})...)
				}
			case "RequestAppRefresh":
				if behavior != nil {
					mockDeps.On("RequestAppRefresh", mock.Anything, mock.Anything).Return(behavior.([]interface{})...)
				}
			case "PersistAppHydratorStatus":
				if behavior != nil {
					mockDeps.On("PersistAppHydratorStatus",
						mock.AnythingOfType("*v1alpha1.Application"),
						mock.MatchedBy(func(status *v1alpha1.SourceHydratorStatus) bool {
							return status.CurrentOperation.Phase == v1alpha1.HydrateOperationPhaseHydrating
						}))
					mockDeps.On("PersistAppHydratorStatus",
						mock.AnythingOfType("*v1alpha1.Application"),
						mock.MatchedBy(func(status *v1alpha1.SourceHydratorStatus) bool {
							return status.CurrentOperation.Phase == behavior.([]interface{})[0].(v1alpha1.HydrateOperationPhase)
						}))
				}
			}
		}
	}

	return hydrator, mockDeps, commitServerMockClientset, mockRepoClientset, repoGetter
}

// setupSuccessfulHydrationMocks configures mocks for a successful hydration scenario
func setupSuccessfulHydrationMocks(app *v1alpha1.Application) map[string]interface{} {
	return map[string]interface{}{
		"GetProcessableApps": []interface{}{&v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{*app},
		}, nil},
		"GetProcessableAppProj": []interface{}{&v1alpha1.AppProject{
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
		}, nil},
		"GetRepoObjs": []interface{}{[]*unstructured.Unstructured{
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
		}, &reposerver.ManifestResponse{}, nil},
		"GetWriteCredentials":      []interface{}{&v1alpha1.Repository{Repo: "https://example.com/repo.git"}, nil},
		"RequestAppRefresh":        []interface{}{nil},
		"PersistAppHydratorStatus": []interface{}{nil},
	}
}

// setupFailedHydrationMocks configures mocks for a failed hydration scenario
func setupFailedHydrationMocks(app *v1alpha1.Application) map[string]interface{} {
	overrides := setupSuccessfulHydrationMocks(app)
	// Override GetRepoObjs to return an error
	overrides["GetRepoObjs"] = []interface{}{[]*unstructured.Unstructured{
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
	}, &reposerver.ManifestResponse{}, errors.New("Obj error")}
	// Remove RequestAppRefresh since it shouldn't be called on failure
	delete(overrides, "RequestAppRefresh")
	return overrides
}

// setupCommitServerMocks configures commit server mocks for successful scenarios
func setupCommitServerMocks(commitClientset *mockcommitclient.Clientset, repoGetter *mocks.RepoGetter, repoClientset *mockrepoclient.Clientset) {
	mockCommitClient := mockcommitclient.CommitServiceClient{}
	mockCommitClient.EXPECT().CommitHydratedManifests(mock.Anything, mock.Anything).Return(&commitclient.CommitHydratedManifestsResponse{HydratedSha: "hydrated-sha"}, error(nil))
	mockCloser := &MockCloser{}
	mockCloser.On("Close").Return(nil)
	commitClientset.EXPECT().NewCommitServerClient().Return(mockCloser, &mockCommitClient, nil)

	repoGetter.On("GetRepository", mock.Anything, "https://example.com/repo.git", "default").Return(&v1alpha1.Repository{Repo: "https://example.com/repo.git"}, nil)
	mockRepoClient := repoClientset.RepoServerServiceClient.(*mockrepoclient.RepoServerServiceClient)
	mockRepoClient.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(&v1alpha1.RevisionMetadata{Message: "some-message"}, nil)
}
func Test_ProcessAppHydrateQueueItem(t *testing.T) {
	now := metav1.NewTime(time.Now())
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))

	log.SetLevel(log.TraceLevel)

	testCases := []struct {
		name                    string
		setupApp                func() *v1alpha1.Application
		setupMocks              func(*v1alpha1.Application) map[string]interface{}
		setupCommitRepoMocks    bool
		expectedPhase           v1alpha1.HydrateOperationPhase
		expectedEvents          []string // Event reasons: "started", "completed", "failed"
		shouldRefresh           bool
		expectedHydratedSHA     string
		shouldCallPersistence   int // Number of times PersistAppHydratorStatus should be called
		shouldCallQueue         int // Number of times AddHydrationQueueItem should be called
		runFullHydrationProcess bool
	}{
		{
			name: "successful complete hydration",
			setupApp: func() *v1alpha1.Application {
				return &v1alpha1.Application{
					Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{DrySource: v1alpha1.DrySource{RepoURL: "https://example.com/repo.git", TargetRevision: "main"}, SyncSource: v1alpha1.SyncSource{Path: "dev", TargetBranch: "dev"}}},
					Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{StartedAt: oneHourAgo}}},
				}
			},
			setupMocks:              setupSuccessfulHydrationMocks,
			setupCommitRepoMocks:    true,
			expectedPhase:           v1alpha1.HydrateOperationPhaseHydrated,
			expectedEvents:          []string{"started", "completed"},
			shouldRefresh:           true,
			expectedHydratedSHA:     "hydrated-sha",
			shouldCallPersistence:   1,
			shouldCallQueue:         1,
			runFullHydrationProcess: true,
		},
		// {
		// 	name: "failed hydration",
		// 	setupApp: func() *v1alpha1.Application {
		// 		return &v1alpha1.Application{
		// 			ObjectMeta: metav1.ObjectMeta{
		// 				Annotations: map[string]string{v1alpha1.AnnotationKeyHydrate: "normal"},
		// 				Name:        "test-app",
		// 				Namespace:   "default",
		// 			},
		// 			Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{DrySource: v1alpha1.DrySource{RepoURL: "https://example.com/repo.git", TargetRevision: "main"}, SyncSource: v1alpha1.SyncSource{Path: "dev", TargetBranch: "dev"}}},
		// 			Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{StartedAt: oneHourAgo}}},
		// 		}
		// 	},
		// 	setupMocks:              setupFailedHydrationMocks,
		// 	setupCommitRepoMocks:    false,
		// 	expectedPhase:           v1alpha1.HydrateOperationPhaseFailed,
		// 	expectedEvents:          []string{"started", "failed"},
		// 	shouldRefresh:           false,
		// 	expectedHydratedSHA:     "",
		// 	shouldCallPersistence:   1,
		// 	shouldCallQueue:         1,
		// 	runFullHydrationProcess: true,
		// },
		// {
		// 	name: "hydration start only",
		// 	setupApp: func() *v1alpha1.Application {
		// 		return &v1alpha1.Application{
		// 			Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
		// 			Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{StartedAt: oneHourAgo}}},
		// 		}
		// 	},
		// 	setupMocks:              func(*v1alpha1.Application) map[string]interface{} { return nil },
		// 	setupCommitRepoMocks:    false,
		// 	expectedPhase:           "", // Phase doesn't change in this scenario
		// 	expectedEvents:          []string{"started"},
		// 	shouldRefresh:           false,
		// 	expectedHydratedSHA:     "",
		// 	shouldCallPersistence:   1,
		// 	shouldCallQueue:         1,
		// 	runFullHydrationProcess: false,
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup application
			app := tc.setupApp()

			// Setup mocks
			mockOverrides := tc.setupMocks(app)
			hydrator, mockDeps, commitClientset, repoClientset, repoGetter := setupCommonMocks(t, mockOverrides)

			// Setup commit/repo server mocks if needed
			if tc.setupCommitRepoMocks {
				setupCommitServerMocks(commitClientset, repoGetter, repoClientset)
			}

			// Setup event logging expectations
			for _, event := range tc.expectedEvents {
				switch event {
				case "started":
					mockDeps.On("LogHydrationPhaseEvent",
						mock.Anything,
						mock.AnythingOfType("*v1alpha1.Application"),
						mock.MatchedBy(func(eventInfo argo.EventInfo) bool {
							return eventInfo.Reason == argo.EventReasonHydrationStarted && eventInfo.Type == corev1.EventTypeNormal
						}),
						"Hydration started").Once()
				case "completed":
					mockDeps.On("LogHydrationPhaseEvent",
						mock.Anything,
						mock.AnythingOfType("*v1alpha1.Application"),
						mock.MatchedBy(func(eventInfo argo.EventInfo) bool {
							return eventInfo.Reason == argo.EventReasonHydrationCompleted && eventInfo.Type == corev1.EventTypeNormal
						}),
						"Hydration completed").Once()
				case "failed":
					mockDeps.On("LogHydrationPhaseEvent",
						mock.Anything,
						mock.AnythingOfType("*v1alpha1.Application"),
						mock.MatchedBy(func(eventInfo argo.EventInfo) bool {
							return eventInfo.Reason == argo.EventReasonHydrationFailed && eventInfo.Type == corev1.EventTypeWarning
						}),
						mock.MatchedBy(func(message string) bool {
							return strings.Contains(message, "Failed to hydrate app")
						})).Once()
				}
			}

			// Override persistence expectations with specific counts
			if tc.shouldCallPersistence > 0 {
				mockDeps.ExpectedCalls = nil // Clear default expectations
				mockDeps.On("PersistAppHydratorStatus", mock.Anything, mock.Anything).Times(tc.shouldCallPersistence)
				mockDeps.On("AddHydrationQueueItem", mock.Anything, mock.Anything).Times(tc.shouldCallQueue)
			}

			hydrator.ProcessAppHydrateQueueItem(app)

			if tc.runFullHydrationProcess && app.Spec.SourceHydrator != nil && app.Spec.SourceHydrator.DrySource.RepoURL != "" {
				hydrator.ProcessHydrationQueueItem(types.HydrationQueueKey{
					SourceRepoURL:        app.Spec.SourceHydrator.DrySource.RepoURL,
					SourceTargetRevision: app.Spec.SourceHydrator.DrySource.TargetRevision,
					DestinationBranch:    app.Spec.SourceHydrator.SyncSource.TargetBranch,
				})
			}

			// 2. Timestamps - StartedAt should always be set after ProcessAppHydrateQueueItem
			assert.NotNil(t, app.Status.SourceHydrator.CurrentOperation.StartedAt,
				"StartedAt timestamp should be set")

			// 3. FinishedAt should be set for completed/failed operations
			if tc.expectedPhase == v1alpha1.HydrateOperationPhaseHydrated || tc.expectedPhase == v1alpha1.HydrateOperationPhaseFailed {
				assert.NotNil(t, app.Status.SourceHydrator.CurrentOperation.FinishedAt,
					"FinishedAt timestamp should be set for completed operations")
			}

			// 4. HydratedSHA validation
			if tc.expectedHydratedSHA != "" {
				assert.Equal(t, tc.expectedHydratedSHA, app.Status.SourceHydrator.CurrentOperation.HydratedSHA,
					"HydratedSHA should match expected value")
			}

			// 5. App refresh validation
			if tc.shouldRefresh {
				mockDeps.AssertCalled(t, "RequestAppRefresh", app.Name, app.Namespace)
			} else {
				mockDeps.AssertNotCalled(t, "RequestAppRefresh", mock.Anything, mock.Anything)
			}

			// 6. Event logging validation - All expected events should have been logged
			// This is validated through the mock expectations set up above

			// 7. Persistence and queue operations validation
			// This is validated through the Times() expectations set up above

			// 8. Verify all mock expectations were met
			mockDeps.AssertExpectations(t)
		})
	}
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
