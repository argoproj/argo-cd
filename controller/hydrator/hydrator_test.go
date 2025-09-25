package hydrator

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/controller/hydrator/mocks"
	"github.com/argoproj/argo-cd/v3/controller/hydrator/types"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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

	d.On("GetProcessableAppProj", mock.Anything).Return(&v1alpha1.AppProject{
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"https://example.com/*"},
		},
	}, nil).Maybe()

	hydrator := &Hydrator{dependencies: d}

	proj, errors := hydrator.validateApplications(apps)
	require.Len(t, errors, 1)
	require.ErrorContains(t, errors[apps[0].QualifiedName()], "App is configured to hydrate to the repository root")
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

// TODO: ProcessAppHydrateQueueItem - currentOperation updated when app needs hydration
// TODO: ProcessAppHydrateQueueItem - request hydration when in progress passed timeout
