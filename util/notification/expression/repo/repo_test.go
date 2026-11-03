package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/util/notification/argocd/mocks"
	"github.com/argoproj/argo-cd/v3/util/notification/expression/shared"
)

func TestGetCommitMetadata(t *testing.T) {
	const sha = "abc123"

	tests := []struct {
		name    string
		spec    map[string]any
		repoURL string
	}{
		{
			name: "single source",
			spec: map[string]any{
				"project": "default",
				"source":  map[string]any{"repoURL": "https://github.com/argoproj/argocd-example-apps.git", "path": "guestbook"},
			},
			repoURL: "https://github.com/argoproj/argocd-example-apps.git",
		},
		{
			name: "empty project resolves to the default project",
			spec: map[string]any{
				"source": map[string]any{"repoURL": "https://github.com/argoproj/argocd-example-apps.git", "path": "guestbook"},
			},
			repoURL: "https://github.com/argoproj/argocd-example-apps.git",
		},
		{
			name: "source hydrator uses the dry source repo",
			spec: map[string]any{
				"project": "default",
				"sourceHydrator": map[string]any{
					"drySource":  map[string]any{"repoURL": "https://github.com/argoproj/argocd-example-apps.git", "targetRevision": "HEAD", "path": "guestbook"},
					"syncSource": map[string]any{"targetBranch": "env/prod", "path": "guestbook"},
				},
			},
			repoURL: "https://github.com/argoproj/argocd-example-apps.git",
		},
		{
			name: "source hydrator uses the sync source repo when set",
			spec: map[string]any{
				"project": "default",
				"sourceHydrator": map[string]any{
					"drySource":  map[string]any{"repoURL": "https://github.com/argoproj/argocd-example-apps.git", "targetRevision": "HEAD", "path": "guestbook"},
					"syncSource": map[string]any{"repoURL": "https://github.com/argoproj/hydrated.git", "targetBranch": "env/prod", "path": "guestbook"},
				},
			},
			repoURL: "https://github.com/argoproj/hydrated.git",
		},
		{
			name: "multiple sources use the first source",
			spec: map[string]any{
				"project": "default",
				"sources": []any{
					map[string]any{"repoURL": "https://github.com/argoproj/argocd-example-apps.git", "path": "guestbook"},
					map[string]any{"repoURL": "https://github.com/argoproj/other.git", "path": "values"},
				},
			},
			repoURL: "https://github.com/argoproj/argocd-example-apps.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata":   map[string]any{"name": "app", "namespace": "argocd"},
				"spec":       tt.spec,
			}}
			argocdService := mocks.NewService(t)
			argocdService.EXPECT().GetCommitMetadata(mock.Anything, tt.repoURL, sha, "default").
				Return(&shared.CommitMetadata{Message: "commit message"}, nil)

			meta, err := getCommitMetadata(sha, app, argocdService)

			require.NoError(t, err)
			assert.Equal(t, "commit message", meta.Message)
		})
	}
}
