package hydrator

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"

	commitclient "github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	commitservermocks "github.com/argoproj/argo-cd/v3/commitserver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v3/controller/hydrator/mocks"
	"github.com/argoproj/argo-cd/v3/controller/hydrator/types"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	reposervermocks "github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

var message = `testn
Argocd-reference-commit-repourl: https://github.com/test/argocd-example-apps
Argocd-reference-commit-author: Argocd-reference-commit-author
Argocd-reference-commit-subject: testhydratormd
Signed-off-by: testUser <test@gmail.com>`

func Test_appNeedsHydration(t *testing.T) {
	t.Parallel()

	now := metav1.NewTime(time.Now())
	oneHourAgo := metav1.NewTime(now.Add(-1 * time.Hour))

	testCases := []struct {
		name                   string
		app                    *v1alpha1.Application
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
			name: "no previous hydrate operation",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
			},
			expectedNeedsHydration: true,
			expectedMessage:        "no previous hydrate operation",
		},
		{
			name: "operation already in progress",
			app: &v1alpha1.Application{
				Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
				Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{Phase: v1alpha1.HydrateOperationPhaseHydrating}}},
			},
			expectedNeedsHydration: false,
			expectedMessage:        "hydration operation already in progress",
		},
		{
			name: "hydrate requested",
			app: &v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1alpha1.AnnotationKeyHydrate: "normal"}},
				Spec:       v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
				Status:     v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{Phase: v1alpha1.HydrateOperationPhaseHydrated}}},
			},
			expectedNeedsHydration: true,
			expectedMessage:        "hydrate requested",
		},
		{
			name: "spec.sourceHydrator differs",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
				Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{
					SourceHydrator: v1alpha1.SourceHydrator{DrySource: v1alpha1.DrySource{RepoURL: "something new"}},
				}}},
			},
			expectedNeedsHydration: true,
			expectedMessage:        "spec.sourceHydrator differs",
		},
		{
			name: "hydration failed more than two minutes ago",
			app: &v1alpha1.Application{
				Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
				Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{DrySHA: "abc123", FinishedAt: &oneHourAgo, Phase: v1alpha1.HydrateOperationPhaseFailed}}},
			},
			expectedNeedsHydration: true,
			expectedMessage:        "previous hydrate operation failed more than 2 minutes ago",
		},
		{
			name: "hydrate not needed",
			app: &v1alpha1.Application{
				Spec:   v1alpha1.ApplicationSpec{SourceHydrator: &v1alpha1.SourceHydrator{}},
				Status: v1alpha1.ApplicationStatus{SourceHydrator: v1alpha1.SourceHydratorStatus{CurrentOperation: &v1alpha1.HydrateOperation{DrySHA: "abc123", StartedAt: now, FinishedAt: &now, Phase: v1alpha1.HydrateOperationPhaseFailed}}},
			},
			expectedNeedsHydration: false,
			expectedMessage:        "hydration not needed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			needsHydration, message := appNeedsHydration(tc.app)
			assert.Equal(t, tc.expectedNeedsHydration, needsHydration)
			assert.Equal(t, tc.expectedMessage, message)
		})
	}
}

func Test_getAppsForHydrationKey_RepoURLNormalization(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	d.EXPECT().GetProcessableApps().Return(&v1alpha1.ApplicationList{
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

	hydrator := &Hydrator{dependencies: d}

	hydrationKey := types.HydrationQueueKey{
		SourceRepoURL:        "https://example.com/repo",
		SourceTargetRevision: "main",
		DestinationBranch:    "main",
	}

	apps, err := hydrator.getAppsForHydrationKey(hydrationKey)

	require.NoError(t, err)
	assert.Len(t, apps, 2, "Expected both apps to be considered relevant despite URL differences")
}

func TestHydrator_getTemplatedCommitMessage(t *testing.T) {
	references := make([]v1alpha1.RevisionReference, 0)
	revReference := v1alpha1.RevisionReference{
		Commit: &v1alpha1.CommitMetadata{
			Author:  "testAuthor",
			Subject: "test",
			RepoURL: "https://github.com/test/argocd-example-apps",
			SHA:     "3ff41cc5247197a6caf50216c4c76cc29d78a97c",
		},
	}
	references = append(references, revReference)
	type args struct {
		repoURL           string
		revision          string
		dryCommitMetadata *v1alpha1.RevisionMetadata
		template          string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test template",
			args: args{
				repoURL:  "https://github.com/test/argocd-example-apps",
				revision: "3ff41cc5247197a6caf50216c4c76cc29d78a97d",
				dryCommitMetadata: &v1alpha1.RevisionMetadata{
					Author: "test test@test.com",
					Date: &metav1.Time{
						Time: metav1.Now().Time,
					},
					Message:    message,
					References: references,
				},
				template: settings.CommitMessageTemplate,
			},
			want: `3ff41cc: testn
Argocd-reference-commit-repourl: https://github.com/test/argocd-example-apps
Argocd-reference-commit-author: Argocd-reference-commit-author
Argocd-reference-commit-subject: testhydratormd
Signed-off-by: testUser <test@gmail.com>

Co-authored-by: testAuthor
Co-authored-by: test test@test.com
`,
		},
		{
			name: "test empty template",
			args: args{
				repoURL:  "https://github.com/test/argocd-example-apps",
				revision: "3ff41cc5247197a6caf50216c4c76cc29d78a97d",
				dryCommitMetadata: &v1alpha1.RevisionMetadata{
					Author: "test test@test.com",
					Date: &metav1.Time{
						Time: metav1.Now().Time,
					},
					Message:    message,
					References: references,
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getTemplatedCommitMessage(tt.args.repoURL, tt.args.revision, tt.args.template, tt.args.dryCommitMetadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("Hydrator.getHydratorCommitMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_validateApplications_RootPathSkipped(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	// create an app that has a SyncSource.Path set to root
	apps := []*v1alpha1.Application{
		{
			Spec: v1alpha1.ApplicationSpec{
				Project: "project",
				SourceHydrator: &v1alpha1.SourceHydrator{
					DrySource: v1alpha1.DrySource{
						RepoURL:        "https://example.com/repo",
						TargetRevision: "main",
						Path:           ".", // root path
					},
					SyncSource: v1alpha1.SyncSource{
						TargetBranch: "main",
						Path:         ".", // root path
					},
				},
			},
		},
	}

	d.EXPECT().GetProcessableAppProj(mock.Anything).Return(&v1alpha1.AppProject{
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"https://example.com/*"},
		},
	}, nil).Maybe()

	hydrator := &Hydrator{dependencies: d}

	proj, errors := hydrator.validateApplications(apps)
	require.Len(t, errors, 1)
	require.ErrorContains(t, errors[apps[0].QualifiedName()], "app is configured to hydrate to the repository root")
	assert.Nil(t, proj)
}

func TestIsRootPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"empty string", "", true},
		{"dot path", ".", true},
		{"slash", string(filepath.Separator), true},
		{"nested path", "app", false},
		{"nested path with slash", "app/", false},
		{"deep path", "app/config", false},
		{"current dir with trailing slash", "./", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRootPath(tt.path)
			require.Equal(t, tt.expected, result)
		})
	}
}

func newTestProject() *v1alpha1.AppProject {
	return &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: "default"},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"https://example.com/repo"},
		},
	}
}

func newTestApp(name string) *v1alpha1.Application {
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: v1alpha1.ApplicationSpec{
			Project: "test-project",
			SourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        "https://example.com/repo",
					TargetRevision: "main",
					Path:           "base/app",
				},
				SyncSource: v1alpha1.SyncSource{
					TargetBranch: "hydrated",
					Path:         "app",
				},
				HydrateTo: &v1alpha1.HydrateTo{
					TargetBranch: "hydrated-next",
				},
			},
		},
	}
	return app
}

func setTestAppPhase(app *v1alpha1.Application, phase v1alpha1.HydrateOperationPhase) *v1alpha1.Application {
	status := v1alpha1.SourceHydratorStatus{}
	switch phase {
	case v1alpha1.HydrateOperationPhaseHydrating:
		status = v1alpha1.SourceHydratorStatus{
			CurrentOperation: &v1alpha1.HydrateOperation{
				StartedAt:      metav1.Now(),
				FinishedAt:     nil,
				Phase:          phase,
				SourceHydrator: *app.Spec.SourceHydrator,
			},
		}
	case v1alpha1.HydrateOperationPhaseFailed:
		status = v1alpha1.SourceHydratorStatus{
			CurrentOperation: &v1alpha1.HydrateOperation{
				StartedAt:      metav1.Now(),
				FinishedAt:     new(metav1.Now()),
				Phase:          phase,
				Message:        "some error",
				SourceHydrator: *app.Spec.SourceHydrator,
			},
		}

	case v1alpha1.HydrateOperationPhaseHydrated:
		status = v1alpha1.SourceHydratorStatus{
			CurrentOperation: &v1alpha1.HydrateOperation{
				StartedAt:      metav1.Now(),
				FinishedAt:     new(metav1.Now()),
				Phase:          phase,
				DrySHA:         "12345",
				HydratedSHA:    "67890",
				SourceHydrator: *app.Spec.SourceHydrator,
			},
		}
	}

	app.Status.SourceHydrator = status
	return app
}

func TestProcessAppHydrateQueueItem_HydrationNeeded(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	app := newTestApp("test-app")

	// appNeedsHydration returns true if no CurrentOperation
	app.Status.SourceHydrator.CurrentOperation = nil

	var persistedStatus *v1alpha1.SourceHydratorStatus
	d.EXPECT().PersistAppHydratorStatus(mock.Anything, mock.Anything).Run(func(_ *v1alpha1.Application, newStatus *v1alpha1.SourceHydratorStatus) {
		persistedStatus = newStatus
	}).Return().Once()
	d.EXPECT().AddHydrationQueueItem(mock.Anything).Return().Once()

	h := &Hydrator{
		dependencies:         d,
		statusRefreshTimeout: time.Minute,
	}

	h.ProcessAppHydrateQueueItem(app)

	d.AssertCalled(t, "PersistAppHydratorStatus", mock.Anything, mock.Anything)
	d.AssertCalled(t, "AddHydrationQueueItem", mock.Anything)

	require.NotNil(t, persistedStatus)
	assert.NotNil(t, persistedStatus.CurrentOperation.StartedAt)
	assert.Nil(t, persistedStatus.CurrentOperation.FinishedAt)
	assert.Equal(t, v1alpha1.HydrateOperationPhaseHydrating, persistedStatus.CurrentOperation.Phase)
	assert.Equal(t, *app.Spec.SourceHydrator, persistedStatus.CurrentOperation.SourceHydrator)
}

func TestProcessAppHydrateQueueItem_HydrationPassedTimeout(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	now := metav1.Now()
	// StartedAt is more than statusRefreshTimeout ago
	startedAt := metav1.NewTime(now.Add(-2 * time.Minute))
	app := newTestApp("test-app")
	app.Status = v1alpha1.ApplicationStatus{
		SourceHydrator: v1alpha1.SourceHydratorStatus{
			CurrentOperation: &v1alpha1.HydrateOperation{
				StartedAt:      startedAt,
				Phase:          v1alpha1.HydrateOperationPhaseHydrating,
				SourceHydrator: v1alpha1.SourceHydrator{},
			},
		},
	}
	d.EXPECT().AddHydrationQueueItem(mock.Anything).Return().Once()

	h := &Hydrator{
		dependencies:         d,
		statusRefreshTimeout: time.Minute,
	}

	h.ProcessAppHydrateQueueItem(app)

	d.AssertCalled(t, "AddHydrationQueueItem", mock.Anything)
	d.AssertNotCalled(t, "PersistAppHydratorStatus", mock.Anything, mock.Anything)
}

func TestProcessAppHydrateQueueItem_NoSourceHydrator(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	app := newTestApp("test-app")
	app.Spec.SourceHydrator = nil

	h := &Hydrator{
		dependencies:         d,
		statusRefreshTimeout: time.Minute,
	}
	h.ProcessAppHydrateQueueItem(app)

	// Should not call anything
	d.AssertNotCalled(t, "PersistAppHydratorStatus", mock.Anything, mock.Anything)
	d.AssertNotCalled(t, "AddHydrationQueueItem", mock.Anything)
}

func TestProcessAppHydrateQueueItem_HydrationNotNeeded(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	now := metav1.Now()
	app := newTestApp("test-app")
	app.Status = v1alpha1.ApplicationStatus{
		SourceHydrator: v1alpha1.SourceHydratorStatus{
			CurrentOperation: &v1alpha1.HydrateOperation{
				StartedAt: now,
				Phase:     v1alpha1.HydrateOperationPhaseHydrating,
			},
		},
	}

	h := &Hydrator{
		dependencies:         d,
		statusRefreshTimeout: time.Minute,
	}
	h.ProcessAppHydrateQueueItem(app)

	// Should not call anything
	d.AssertNotCalled(t, "PersistAppHydratorStatus", mock.Anything, mock.Anything)
	d.AssertNotCalled(t, "AddHydrationQueueItem", mock.Anything)
}

func TestProcessHydrationQueueItem_ValidationFails(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	app1 := setTestAppPhase(newTestApp("test-app"), v1alpha1.HydrateOperationPhaseHydrating)
	app2 := setTestAppPhase(newTestApp("test-app-2"), v1alpha1.HydrateOperationPhaseHydrating)
	hydrationKey := getHydrationQueueKey(app1)

	// getAppsForHydrationKey returns two apps
	d.EXPECT().GetProcessableApps().Return(&v1alpha1.ApplicationList{Items: []v1alpha1.Application{*app1, *app2}}, nil)
	d.EXPECT().GetProcessableAppProj(mock.Anything).Return(nil, errors.New("test error")).Once()
	d.EXPECT().GetProcessableAppProj(mock.Anything).Return(newTestProject(), nil).Once()

	h := &Hydrator{dependencies: d}

	// Expect setAppHydratorError to be called
	var persistedStatus1 *v1alpha1.SourceHydratorStatus
	var persistedStatus2 *v1alpha1.SourceHydratorStatus
	d.EXPECT().PersistAppHydratorStatus(mock.Anything, mock.Anything).Run(func(orig *v1alpha1.Application, newStatus *v1alpha1.SourceHydratorStatus) {
		switch orig.Name {
		case app1.Name:
			persistedStatus1 = newStatus
		case app2.Name:
			persistedStatus2 = newStatus
		}
	}).Return().Twice()

	h.ProcessHydrationQueueItem(hydrationKey)

	assert.NotNil(t, persistedStatus1)
	assert.NotNil(t, persistedStatus1.CurrentOperation.FinishedAt)
	assert.Contains(t, persistedStatus1.CurrentOperation.Message, "test error")
	assert.Equal(t, v1alpha1.HydrateOperationPhaseFailed, persistedStatus1.CurrentOperation.Phase)
	assert.NotNil(t, persistedStatus2)
	assert.NotNil(t, persistedStatus2.CurrentOperation.FinishedAt)
	assert.Contains(t, persistedStatus2.CurrentOperation.Message, "cannot hydrate because application default/test-app has an error")
	assert.Equal(t, v1alpha1.HydrateOperationPhaseFailed, persistedStatus1.CurrentOperation.Phase)

	d.AssertNumberOfCalls(t, "PersistAppHydratorStatus", 2)
	d.AssertNotCalled(t, "RequestAppRefresh", mock.Anything, mock.Anything)
}

func TestProcessHydrationQueueItem_HydrateFails_AppSpecificError(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	app1 := setTestAppPhase(newTestApp("test-app"), v1alpha1.HydrateOperationPhaseHydrating)
	app2 := newTestApp("test-app-2")
	app2.Spec.SourceHydrator.SyncSource.Path = "something/else"
	app2 = setTestAppPhase(app2, v1alpha1.HydrateOperationPhaseHydrating)
	hydrationKey := getHydrationQueueKey(app1)

	d.EXPECT().GetProcessableApps().Return(&v1alpha1.ApplicationList{Items: []v1alpha1.Application{*app1, *app2}}, nil)
	d.EXPECT().GetProcessableAppProj(mock.Anything).Return(newTestProject(), nil)

	h := &Hydrator{dependencies: d}

	// Make hydrate return app-specific error
	d.EXPECT().GetRepoObjs(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, errors.New("hydrate error"))

	// Expect setAppHydratorError to be called
	var persistedStatus1 *v1alpha1.SourceHydratorStatus
	var persistedStatus2 *v1alpha1.SourceHydratorStatus
	d.EXPECT().PersistAppHydratorStatus(mock.Anything, mock.Anything).Run(func(orig *v1alpha1.Application, newStatus *v1alpha1.SourceHydratorStatus) {
		switch orig.Name {
		case app1.Name:
			persistedStatus1 = newStatus
		case app2.Name:
			persistedStatus2 = newStatus
		}
	}).Return().Twice()

	h.ProcessHydrationQueueItem(hydrationKey)

	assert.NotNil(t, persistedStatus1)
	assert.NotNil(t, persistedStatus1.CurrentOperation.FinishedAt)
	assert.Contains(t, persistedStatus1.CurrentOperation.Message, "hydrate error")
	assert.Equal(t, v1alpha1.HydrateOperationPhaseFailed, persistedStatus1.CurrentOperation.Phase)
	assert.NotNil(t, persistedStatus2)
	assert.NotNil(t, persistedStatus2.CurrentOperation.FinishedAt)
	assert.Contains(t, persistedStatus2.CurrentOperation.Message, "cannot hydrate because application default/test-app has an error")
	assert.Equal(t, v1alpha1.HydrateOperationPhaseFailed, persistedStatus1.CurrentOperation.Phase)

	d.AssertNumberOfCalls(t, "PersistAppHydratorStatus", 2)
	d.AssertNotCalled(t, "RequestAppRefresh", mock.Anything, mock.Anything)
}

func TestProcessHydrationQueueItem_HydrateFails_CommonError(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	r := mocks.NewRepoGetter(t)
	app1 := setTestAppPhase(newTestApp("test-app"), v1alpha1.HydrateOperationPhaseHydrating)
	app2 := newTestApp("test-app-2")
	app2.Spec.SourceHydrator.SyncSource.Path = "something/else"
	app2 = setTestAppPhase(app2, v1alpha1.HydrateOperationPhaseHydrating)
	hydrationKey := getHydrationQueueKey(app1)
	d.EXPECT().GetProcessableApps().Return(&v1alpha1.ApplicationList{Items: []v1alpha1.Application{*app1, *app2}}, nil)
	d.EXPECT().GetProcessableAppProj(mock.Anything).Return(newTestProject(), nil)
	h := &Hydrator{dependencies: d, repoGetter: r}

	d.EXPECT().GetRepoObjs(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, &repoclient.ManifestResponse{
		Revision: "abc123",
	}, nil)
	r.EXPECT().GetRepository(mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("repo error"))

	// Expect setAppHydratorError to be called
	var persistedStatus1 *v1alpha1.SourceHydratorStatus
	var persistedStatus2 *v1alpha1.SourceHydratorStatus
	d.EXPECT().PersistAppHydratorStatus(mock.Anything, mock.Anything).Run(func(orig *v1alpha1.Application, newStatus *v1alpha1.SourceHydratorStatus) {
		switch orig.Name {
		case app1.Name:
			persistedStatus1 = newStatus
		case app2.Name:
			persistedStatus2 = newStatus
		}
	}).Return().Twice()

	h.ProcessHydrationQueueItem(hydrationKey)

	assert.NotNil(t, persistedStatus1)
	assert.NotNil(t, persistedStatus1.CurrentOperation.FinishedAt)
	assert.Contains(t, persistedStatus1.CurrentOperation.Message, "repo error")
	assert.Equal(t, v1alpha1.HydrateOperationPhaseFailed, persistedStatus1.CurrentOperation.Phase)
	assert.Equal(t, "abc123", persistedStatus1.CurrentOperation.DrySHA)
	assert.NotNil(t, persistedStatus2)
	assert.NotNil(t, persistedStatus2.CurrentOperation.FinishedAt)
	assert.Contains(t, persistedStatus2.CurrentOperation.Message, "repo error")
	assert.Equal(t, v1alpha1.HydrateOperationPhaseFailed, persistedStatus1.CurrentOperation.Phase)
	assert.Equal(t, "abc123", persistedStatus1.CurrentOperation.DrySHA)

	d.AssertNumberOfCalls(t, "PersistAppHydratorStatus", 2)
	d.AssertNotCalled(t, "RequestAppRefresh", mock.Anything, mock.Anything)
}

func TestProcessHydrationQueueItem_SuccessfulHydration(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	r := mocks.NewRepoGetter(t)
	rc := reposervermocks.NewRepoServerServiceClient(t)
	cc := commitservermocks.NewCommitServiceClient(t)
	app := setTestAppPhase(newTestApp("test-app"), v1alpha1.HydrateOperationPhaseHydrating)
	hydrationKey := getHydrationQueueKey(app)
	d.EXPECT().GetProcessableApps().Return(&v1alpha1.ApplicationList{Items: []v1alpha1.Application{*app}}, nil)
	d.EXPECT().GetProcessableAppProj(mock.Anything).Return(newTestProject(), nil)
	h := &Hydrator{dependencies: d, repoGetter: r, commitClientset: &commitservermocks.Clientset{CommitServiceClient: cc}, repoClientset: &reposervermocks.Clientset{RepoServerServiceClient: rc}}

	// Expect setAppHydratorError to be called
	var persistedStatus *v1alpha1.SourceHydratorStatus
	d.EXPECT().PersistAppHydratorStatus(mock.Anything, mock.Anything).Run(func(_ *v1alpha1.Application, newStatus *v1alpha1.SourceHydratorStatus) {
		persistedStatus = newStatus
	}).Return().Once()
	d.EXPECT().RequestAppRefresh(app.Name, app.Namespace).Return(nil).Once()
	d.EXPECT().GetRepoObjs(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, &repoclient.ManifestResponse{
		Revision: "abc123",
	}, nil).Once()
	r.EXPECT().GetRepository(mock.Anything, "https://example.com/repo", "test-project").Return(nil, nil).Once()
	rc.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(nil, nil).Once()
	d.EXPECT().GetWriteCredentials(mock.Anything, "https://example.com/repo", "test-project").Return(nil, nil).Once()
	d.EXPECT().GetHydratorCommitMessageTemplate().Return("commit message", nil).Once()
	d.EXPECT().GetCommitAuthorName().Return("", nil).Once()
	d.EXPECT().GetCommitAuthorEmail().Return("", nil).Once()
	cc.EXPECT().CommitHydratedManifests(mock.Anything, mock.Anything).Return(&commitclient.CommitHydratedManifestsResponse{HydratedSha: "def456"}, nil).Once()

	h.ProcessHydrationQueueItem(hydrationKey)

	d.AssertCalled(t, "PersistAppHydratorStatus", mock.Anything, mock.Anything)
	d.AssertCalled(t, "RequestAppRefresh", app.Name, app.Namespace)
	assert.NotNil(t, persistedStatus)
	assert.Equal(t, app.Status.SourceHydrator.CurrentOperation.StartedAt, persistedStatus.CurrentOperation.StartedAt)
	assert.Equal(t, app.Status.SourceHydrator.CurrentOperation.SourceHydrator, persistedStatus.CurrentOperation.SourceHydrator)
	assert.NotNil(t, persistedStatus.CurrentOperation.FinishedAt)
	assert.Equal(t, v1alpha1.HydrateOperationPhaseHydrated, persistedStatus.CurrentOperation.Phase)
	assert.Empty(t, persistedStatus.CurrentOperation.Message)
	assert.Equal(t, "abc123", persistedStatus.CurrentOperation.DrySHA)
	assert.Equal(t, "def456", persistedStatus.CurrentOperation.HydratedSHA)
	assert.NotNil(t, persistedStatus.LastSuccessfulOperation)
	assert.Equal(t, "abc123", persistedStatus.LastSuccessfulOperation.DrySHA)
	assert.Equal(t, "def456", persistedStatus.LastSuccessfulOperation.HydratedSHA)
	assert.Equal(t, app.Status.SourceHydrator.CurrentOperation.SourceHydrator, persistedStatus.LastSuccessfulOperation.SourceHydrator)
}

func TestValidateApplications_ProjectError(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	app := newTestApp("test-app")
	d.EXPECT().GetProcessableAppProj(app).Return(nil, errors.New("project error")).Once()
	h := &Hydrator{dependencies: d}

	projects, errs := h.validateApplications([]*v1alpha1.Application{app})
	require.Nil(t, projects)
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[app.QualifiedName()], "project error")
}

func TestValidateApplications_SourceNotPermitted(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	app := newTestApp("test-app")
	proj := newTestProject()
	proj.Spec.SourceRepos = []string{"not-allowed"}
	d.EXPECT().GetProcessableAppProj(app).Return(proj, nil).Once()
	h := &Hydrator{dependencies: d}

	projects, errs := h.validateApplications([]*v1alpha1.Application{app})
	require.Nil(t, projects)
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[app.QualifiedName()], "application repo https://example.com/repo is not permitted in project 'test-project'")
}

func TestValidateApplications_RootPath(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	app := newTestApp("test-app")
	app.Spec.SourceHydrator.SyncSource.Path = "."
	proj := newTestProject()
	d.EXPECT().GetProcessableAppProj(app).Return(proj, nil).Once()
	h := &Hydrator{dependencies: d}

	projects, errs := h.validateApplications([]*v1alpha1.Application{app})
	require.Nil(t, projects)
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[app.QualifiedName()], "app is configured to hydrate to the repository root")
}

func TestValidateApplications_DuplicateDestination(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	app1 := newTestApp("app1")
	app2 := newTestApp("app2")
	app2.Spec.SourceHydrator.SyncSource.Path = app1.Spec.SourceHydrator.SyncSource.Path // duplicate path
	proj := newTestProject()
	d.EXPECT().GetProcessableAppProj(app1).Return(proj, nil).Once()
	d.EXPECT().GetProcessableAppProj(app2).Return(proj, nil).Once()
	h := &Hydrator{dependencies: d}

	projects, errs := h.validateApplications([]*v1alpha1.Application{app1, app2})
	require.Nil(t, projects)
	require.Len(t, errs, 2)
	require.ErrorContains(t, errs[app1.QualifiedName()], "app default/app2 hydrator use the same destination")
	require.ErrorContains(t, errs[app2.QualifiedName()], "app default/app1 hydrator use the same destination")
}

func TestValidateApplications_Success(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	app1 := newTestApp("app1")
	app2 := newTestApp("app2")
	app2.Spec.SourceHydrator.SyncSource.Path = "other-path"
	proj := newTestProject()
	d.EXPECT().GetProcessableAppProj(app1).Return(proj, nil).Once()
	d.EXPECT().GetProcessableAppProj(app2).Return(proj, nil).Once()
	h := &Hydrator{dependencies: d}

	projects, errs := h.validateApplications([]*v1alpha1.Application{app1, app2})
	require.NotNil(t, projects)
	require.Empty(t, errs)
	assert.Equal(t, proj, projects[app1.Spec.Project])
	assert.Equal(t, proj, projects[app2.Spec.Project])
}

func TestGenericHydrationError(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		err := genericHydrationError(map[string]error{})
		assert.NoError(t, err)
	})

	t.Run("single error", func(t *testing.T) {
		errs := map[string]error{
			"default/app1": errors.New("error1"),
		}
		err := genericHydrationError(errs)
		require.Error(t, err)
		assert.Equal(t, "cannot hydrate because application default/app1 has an error", err.Error())
	})

	t.Run("multiple errors", func(t *testing.T) {
		errs := map[string]error{
			"default/app1": errors.New("error1"),
			"default/app2": errors.New("error2"),
			"default/app3": errors.New("error3"),
		}
		err := genericHydrationError(errs)
		require.Error(t, err)
		// Sorted keys, so default/app1 is first
		assert.Equal(t, "cannot hydrate because application default/app1 and 2 more have errors", err.Error())
	})
}

func TestHydrator_hydrate_Success(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	r := mocks.NewRepoGetter(t)
	cc := commitservermocks.NewCommitServiceClient(t)
	rc := reposervermocks.NewRepoServerServiceClient(t)
	h := &Hydrator{
		dependencies:    d,
		repoGetter:      r,
		repoClientset:   &reposervermocks.Clientset{RepoServerServiceClient: rc},
		commitClientset: &commitservermocks.Clientset{CommitServiceClient: cc},
	}

	app1 := newTestApp("app1")
	app2 := newTestApp("app2")
	app2.Spec.SourceHydrator.SyncSource.Path = "other-path"
	apps := []*v1alpha1.Application{app1, app2}
	proj := newTestProject()
	projects := map[string]*v1alpha1.AppProject{app1.Spec.Project: proj}
	readRepo := &v1alpha1.Repository{Repo: "https://example.com/repo"}
	writeRepo := &v1alpha1.Repository{Repo: "https://example.com/repo"}

	d.EXPECT().GetRepoObjs(mock.Anything, app1, app1.Spec.SourceHydrator.GetDrySource(), "main", proj).Return(nil, &repoclient.ManifestResponse{Revision: "sha123"}, nil)
	d.EXPECT().GetRepoObjs(mock.Anything, app2, app2.Spec.SourceHydrator.GetDrySource(), "sha123", proj).Return(nil, &repoclient.ManifestResponse{Revision: "sha123"}, nil)
	r.EXPECT().GetRepository(mock.Anything, readRepo.Repo, proj.Name).Return(readRepo, nil)
	rc.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(&v1alpha1.RevisionMetadata{Message: "metadata"}, nil).Run(func(_ context.Context, in *repoclient.RepoServerRevisionMetadataRequest, _ ...grpc.CallOption) {
		assert.Equal(t, readRepo, in.Repo)
		assert.Equal(t, "sha123", in.Revision)
	})
	d.EXPECT().GetWriteCredentials(mock.Anything, readRepo.Repo, proj.Name).Return(writeRepo, nil)
	d.EXPECT().GetHydratorCommitMessageTemplate().Return("commit message", nil)
	d.EXPECT().GetCommitAuthorName().Return("", nil)
	d.EXPECT().GetCommitAuthorEmail().Return("", nil)
	cc.EXPECT().CommitHydratedManifests(mock.Anything, mock.Anything).Return(&commitclient.CommitHydratedManifestsResponse{HydratedSha: "hydrated123"}, nil).Run(func(_ context.Context, in *commitclient.CommitHydratedManifestsRequest, _ ...grpc.CallOption) {
		assert.Equal(t, "commit message", in.CommitMessage)
		assert.Equal(t, "hydrated", in.SyncBranch)
		assert.Equal(t, "hydrated-next", in.TargetBranch)
		assert.Equal(t, "sha123", in.DrySha)
		assert.Equal(t, writeRepo, in.Repo)
		assert.Len(t, in.Paths, 2)
		assert.Equal(t, app1.Spec.SourceHydrator.SyncSource.Path, in.Paths[0].Path)
		assert.Equal(t, app2.Spec.SourceHydrator.SyncSource.Path, in.Paths[1].Path)
		assert.Equal(t, "metadata", in.DryCommitMetadata.Message)
	})
	logCtx := log.NewEntry(log.StandardLogger())

	sha, hydratedSha, errs, err := h.hydrate(logCtx, apps, projects)

	require.NoError(t, err)
	assert.Equal(t, "sha123", sha)
	assert.Equal(t, "hydrated123", hydratedSha)
	assert.Empty(t, errs)
}

func TestHydrator_hydrate_GetManifestsError(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	r := mocks.NewRepoGetter(t)
	cc := commitservermocks.NewCommitServiceClient(t)
	rc := reposervermocks.NewRepoServerServiceClient(t)
	h := &Hydrator{
		dependencies:    d,
		repoGetter:      r,
		repoClientset:   &reposervermocks.Clientset{RepoServerServiceClient: rc},
		commitClientset: &commitservermocks.Clientset{CommitServiceClient: cc},
	}

	app := newTestApp("app1")
	proj := newTestProject()
	projects := map[string]*v1alpha1.AppProject{app.Spec.Project: proj}

	d.EXPECT().GetRepoObjs(mock.Anything, app, mock.Anything, mock.Anything, proj).Return(nil, nil, errors.New("manifests error"))
	logCtx := log.NewEntry(log.StandardLogger())

	sha, hydratedSha, errs, err := h.hydrate(logCtx, []*v1alpha1.Application{app}, projects)

	require.NoError(t, err)
	assert.Empty(t, sha)
	assert.Empty(t, hydratedSha)
	require.Len(t, errs, 1)
	assert.ErrorContains(t, errs[app.QualifiedName()], "manifests error")
}

func TestHydrator_hydrate_RevisionMetadataError(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	r := mocks.NewRepoGetter(t)
	cc := commitservermocks.NewCommitServiceClient(t)
	rc := reposervermocks.NewRepoServerServiceClient(t)
	h := &Hydrator{
		dependencies:    d,
		repoGetter:      r,
		repoClientset:   &reposervermocks.Clientset{RepoServerServiceClient: rc},
		commitClientset: &commitservermocks.Clientset{CommitServiceClient: cc},
	}

	app := newTestApp("app1")
	proj := newTestProject()
	projects := map[string]*v1alpha1.AppProject{app.Spec.Project: proj}

	d.EXPECT().GetRepoObjs(mock.Anything, app, mock.Anything, mock.Anything, proj).Return(nil, &repoclient.ManifestResponse{Revision: "sha123"}, nil)
	r.EXPECT().GetRepository(mock.Anything, mock.Anything, mock.Anything).Return(&v1alpha1.Repository{Repo: "https://example.com/repo"}, nil)
	rc.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(nil, errors.New("metadata error"))
	logCtx := log.NewEntry(log.StandardLogger())

	sha, hydratedSha, errs, err := h.hydrate(logCtx, []*v1alpha1.Application{app}, projects)

	require.Error(t, err)
	assert.Equal(t, "sha123", sha)
	assert.Empty(t, hydratedSha)
	assert.Empty(t, errs)
	assert.ErrorContains(t, err, "metadata error")
}

func TestHydrator_hydrate_GetWriteCredentialsError(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	r := mocks.NewRepoGetter(t)
	cc := commitservermocks.NewCommitServiceClient(t)
	rc := reposervermocks.NewRepoServerServiceClient(t)
	h := &Hydrator{
		dependencies:    d,
		repoGetter:      r,
		repoClientset:   &reposervermocks.Clientset{RepoServerServiceClient: rc},
		commitClientset: &commitservermocks.Clientset{CommitServiceClient: cc},
	}

	app := newTestApp("app1")
	proj := newTestProject()
	projects := map[string]*v1alpha1.AppProject{app.Spec.Project: proj}

	d.EXPECT().GetRepoObjs(mock.Anything, app, mock.Anything, mock.Anything, proj).Return(nil, &repoclient.ManifestResponse{Revision: "sha123"}, nil)
	r.EXPECT().GetRepository(mock.Anything, mock.Anything, mock.Anything).Return(&v1alpha1.Repository{Repo: "https://example.com/repo"}, nil)
	rc.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(&v1alpha1.RevisionMetadata{}, nil)
	d.EXPECT().GetWriteCredentials(mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("creds error"))
	logCtx := log.NewEntry(log.StandardLogger())

	sha, hydratedSha, errs, err := h.hydrate(logCtx, []*v1alpha1.Application{app}, projects)

	require.Error(t, err)
	assert.Equal(t, "sha123", sha)
	assert.Empty(t, hydratedSha)
	assert.Empty(t, errs)
	assert.ErrorContains(t, err, "creds error")
}

func TestHydrator_hydrate_CommitMessageTemplateError(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	r := mocks.NewRepoGetter(t)
	cc := commitservermocks.NewCommitServiceClient(t)
	rc := reposervermocks.NewRepoServerServiceClient(t)
	h := &Hydrator{
		dependencies:    d,
		repoGetter:      r,
		repoClientset:   &reposervermocks.Clientset{RepoServerServiceClient: rc},
		commitClientset: &commitservermocks.Clientset{CommitServiceClient: cc},
	}

	app := newTestApp("app1")
	proj := newTestProject()
	projects := map[string]*v1alpha1.AppProject{app.Spec.Project: proj}

	d.EXPECT().GetRepoObjs(mock.Anything, app, mock.Anything, mock.Anything, proj).Return(nil, &repoclient.ManifestResponse{Revision: "sha123"}, nil)
	r.EXPECT().GetRepository(mock.Anything, mock.Anything, mock.Anything).Return(&v1alpha1.Repository{Repo: "https://example.com/repo"}, nil)
	rc.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(&v1alpha1.RevisionMetadata{}, nil)
	d.EXPECT().GetWriteCredentials(mock.Anything, mock.Anything, mock.Anything).Return(&v1alpha1.Repository{Repo: "https://example.com/repo"}, nil)
	d.EXPECT().GetHydratorCommitMessageTemplate().Return("", errors.New("template error"))
	logCtx := log.NewEntry(log.StandardLogger())

	sha, hydratedSha, errs, err := h.hydrate(logCtx, []*v1alpha1.Application{app}, projects)

	require.Error(t, err)
	assert.Equal(t, "sha123", sha)
	assert.Empty(t, hydratedSha)
	assert.Empty(t, errs)
	assert.ErrorContains(t, err, "template error")
}

func TestHydrator_hydrate_TemplatedCommitMessageError(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	r := mocks.NewRepoGetter(t)
	cc := commitservermocks.NewCommitServiceClient(t)
	rc := reposervermocks.NewRepoServerServiceClient(t)
	h := &Hydrator{
		dependencies:    d,
		repoGetter:      r,
		repoClientset:   &reposervermocks.Clientset{RepoServerServiceClient: rc},
		commitClientset: &commitservermocks.Clientset{CommitServiceClient: cc},
	}

	app := newTestApp("app1")
	proj := newTestProject()
	projects := map[string]*v1alpha1.AppProject{app.Spec.Project: proj}

	d.EXPECT().GetRepoObjs(mock.Anything, app, mock.Anything, mock.Anything, proj).Return(nil, &repoclient.ManifestResponse{Revision: "sha123"}, nil)
	r.EXPECT().GetRepository(mock.Anything, mock.Anything, mock.Anything).Return(&v1alpha1.Repository{Repo: "https://example.com/repo"}, nil)
	rc.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(&v1alpha1.RevisionMetadata{}, nil)
	d.EXPECT().GetWriteCredentials(mock.Anything, mock.Anything, mock.Anything).Return(&v1alpha1.Repository{Repo: "https://example.com/repo"}, nil)
	d.EXPECT().GetHydratorCommitMessageTemplate().Return("{{ notAFunction }} template", nil)
	logCtx := log.NewEntry(log.StandardLogger())

	sha, hydratedSha, errs, err := h.hydrate(logCtx, []*v1alpha1.Application{app}, projects)

	require.Error(t, err)
	assert.Equal(t, "sha123", sha)
	assert.Empty(t, hydratedSha)
	assert.Empty(t, errs)
	assert.ErrorContains(t, err, "failed to parse template")
}

func TestHydrator_hydrate_CommitHydratedManifestsError(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	r := mocks.NewRepoGetter(t)
	cc := commitservermocks.NewCommitServiceClient(t)
	rc := reposervermocks.NewRepoServerServiceClient(t)
	h := &Hydrator{
		dependencies:    d,
		repoGetter:      r,
		repoClientset:   &reposervermocks.Clientset{RepoServerServiceClient: rc},
		commitClientset: &commitservermocks.Clientset{CommitServiceClient: cc},
	}

	app := newTestApp("app1")
	proj := newTestProject()
	projects := map[string]*v1alpha1.AppProject{app.Spec.Project: proj}

	d.EXPECT().GetRepoObjs(mock.Anything, app, mock.Anything, mock.Anything, proj).Return(nil, &repoclient.ManifestResponse{Revision: "sha123"}, nil)
	r.EXPECT().GetRepository(mock.Anything, mock.Anything, mock.Anything).Return(&v1alpha1.Repository{Repo: "https://example.com/repo"}, nil)
	rc.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(&v1alpha1.RevisionMetadata{}, nil)
	d.EXPECT().GetWriteCredentials(mock.Anything, mock.Anything, mock.Anything).Return(&v1alpha1.Repository{Repo: "https://example.com/repo"}, nil)
	d.EXPECT().GetHydratorCommitMessageTemplate().Return("commit message", nil)
	d.EXPECT().GetCommitAuthorName().Return("", nil)
	d.EXPECT().GetCommitAuthorEmail().Return("", nil)
	cc.EXPECT().CommitHydratedManifests(mock.Anything, mock.Anything).Return(nil, errors.New("commit error"))
	logCtx := log.NewEntry(log.StandardLogger())

	sha, hydratedSha, errs, err := h.hydrate(logCtx, []*v1alpha1.Application{app}, projects)

	require.Error(t, err)
	assert.Equal(t, "sha123", sha)
	assert.Empty(t, hydratedSha)
	assert.Empty(t, errs)
	assert.ErrorContains(t, err, "commit error")
}

func TestHydrator_hydrate_EmptyApps(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	logCtx := log.NewEntry(log.StandardLogger())
	h := &Hydrator{dependencies: d}

	sha, hydratedSha, errs, err := h.hydrate(logCtx, []*v1alpha1.Application{}, nil)

	require.NoError(t, err)
	assert.Empty(t, sha)
	assert.Empty(t, hydratedSha)
	assert.Empty(t, errs)
}

func TestHydrator_getManifests_Success(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	h := &Hydrator{dependencies: d}
	app := newTestApp("test-app")
	proj := newTestProject()

	cm := kube.MustToUnstructured(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	})

	d.EXPECT().GetRepoObjs(mock.Anything, app, app.Spec.SourceHydrator.GetDrySource(), "sha123", proj).Return([]*unstructured.Unstructured{cm}, &repoclient.ManifestResponse{
		Revision: "sha123",
		Commands: []string{"cmd1", "cmd2"},
	}, nil)

	rev, pathDetails, err := h.getManifests(t.Context(), app, "sha123", proj)
	require.NoError(t, err)
	assert.Equal(t, "sha123", rev)
	assert.Equal(t, app.Spec.SourceHydrator.SyncSource.Path, pathDetails.Path)
	assert.Equal(t, []string{"cmd1", "cmd2"}, pathDetails.Commands)
	assert.Len(t, pathDetails.Manifests, 1)
	assert.JSONEq(t, `{"metadata":{"name":"test"}}`, pathDetails.Manifests[0].ManifestJSON)
}

func TestHydrator_getManifests_EmptyTargetRevision(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	h := &Hydrator{dependencies: d}
	app := newTestApp("test-app")
	proj := newTestProject()

	d.EXPECT().GetRepoObjs(mock.Anything, app, mock.Anything, "main", proj).Return([]*unstructured.Unstructured{}, &repoclient.ManifestResponse{Revision: "sha123"}, nil)

	rev, pathDetails, err := h.getManifests(t.Context(), app, "", proj)
	require.NoError(t, err)
	assert.Equal(t, "sha123", rev)
	assert.NotNil(t, pathDetails)
}

func TestHydrator_getManifests_GetRepoObjsError(t *testing.T) {
	t.Parallel()
	d := mocks.NewDependencies(t)
	h := &Hydrator{dependencies: d}
	app := newTestApp("test-app")
	proj := newTestProject()

	d.EXPECT().GetRepoObjs(mock.Anything, app, mock.Anything, "main", proj).Return(nil, nil, errors.New("repo error"))

	rev, pathDetails, err := h.getManifests(t.Context(), app, "main", proj)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo error")
	assert.Empty(t, rev)
	assert.Nil(t, pathDetails)
}

func TestHydrator_hydrate_DeDupe_Success(t *testing.T) {
	t.Parallel()

	d := mocks.NewDependencies(t)
	h := &Hydrator{dependencies: d}

	app1 := newTestApp("app1")
	app2 := newTestApp("app2")
	lastSuccessfulOperation := &v1alpha1.SuccessfulHydrateOperation{
		DrySHA:      "sha123",
		HydratedSHA: "hydrated123",
	}
	app1.Status.SourceHydrator = v1alpha1.SourceHydratorStatus{
		LastSuccessfulOperation: lastSuccessfulOperation,
	}

	apps := []*v1alpha1.Application{app1, app2}
	proj := newTestProject()
	projects := map[string]*v1alpha1.AppProject{app1.Spec.Project: proj}

	// Asserting .Once() confirms that we only make one call to repo-server to get the last hydrated DRY
	// sha, and then we quit early.
	d.On("GetRepoObjs", mock.Anything, app1, app1.Spec.SourceHydrator.GetDrySource(), "main", proj).Return(nil, &repoclient.ManifestResponse{Revision: "sha123"}, nil).Once()
	logCtx := log.NewEntry(log.StandardLogger())

	sha, hydratedSha, errs, err := h.hydrate(logCtx, apps, projects)

	require.NoError(t, err)
	assert.Equal(t, "sha123", sha)
	assert.Equal(t, "hydrated123", hydratedSha)
	assert.Empty(t, errs)
}
