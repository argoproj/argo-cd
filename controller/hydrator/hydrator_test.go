package hydrator

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/controller/hydrator/mocks"
	"github.com/argoproj/argo-cd/v3/controller/hydrator/types"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var message = `testn
Argocd-reference-commit-repourl: https://github.com/test/argocd-example-apps
Argocd-reference-commit-author: Argocd-reference-commit-author
Argocd-reference-commit-subject: testhydratormd
Signed-off-by: testUser <test@gmail.com>`

var commitMessageTemplate = `    {{- if .metadata }}
        {{- if .metadata.repoURL }}
            repoURL: {{ .metadata.repoURL }}
        {{- end }}
        
        {{- if .metadata.drySha }}
            drySha: {{ .metadata.drySha }}
        {{- end }}

        {{- if .metadata.author }}
            Co-authored-by: {{ .metadata.author }}
        {{- end }}

        {{- if .metadata.subject }}
            subject: {{ .metadata.subject }}
        {{- end }}

        {{- if .metadata.body }}
            body: {{ .metadata.body }}
        {{- end }}
        {{- if .metadata.references }}
            References:
            {{- range $reference := .metadata.references }}
                {{- if kindIs "map" $reference.commit }}
                    Commit:
                    {{- range $key, $value := $reference.commit }}
                        {{- if eq $key "author" }}
                            Co-authored-by: {{ $value }}
                        {{- end }}
                    {{- end }}
                {{- end }}
            {{- end }}
        {{- end }}
    {{- end }} 
`

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

func TestHydrator_getHydratorCommitMessage(t *testing.T) {
	d := mocks.NewDependencies(t)
	d.On("GetHydratorCommitMessageTemplate").Return(commitMessageTemplate, nil)
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hydrator{
				dependencies: d,
			}
			got, err := h.getHydratorCommitMessage(tt.args.repoURL, tt.args.revision, tt.args.dryCommitMetadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("Hydrator.getHydratorCommitMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == "" {
				t.Errorf("Hydrator.getHydratorCommitMessage() = %v, want %v", got, tt.want)
			}
			assert.NotEmpty(t, got)
			assert.Contains(t, got, "Argocd-reference")
		})
	}
}
