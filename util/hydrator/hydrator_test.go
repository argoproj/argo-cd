package hydrator

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestGetDrySHA(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer root.Close()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "app"), 0o700))
	metadataPath := filepath.Join(dir, "app", "hydrator.metadata")

	assert.Empty(t, GetDrySHA(root, "app"))

	require.NoError(t, os.WriteFile(metadataPath, []byte(`{"repoURL":"https://example.com/repo","drySha":"abc123"}`), 0o600))
	assert.Equal(t, "abc123", GetDrySHA(root, "app"))
	assert.Empty(t, GetDrySHA(root, ""), "the root file is separate from the app's")

	require.NoError(t, os.WriteFile(metadataPath, []byte("{"), 0o600))
	assert.Empty(t, GetDrySHA(root, "app"))
}

func TestGetCommitMetadata(t *testing.T) {
	t.Parallel()
	repoURL := "https://github.com/test/argocd-example-apps"
	drySHA := "3ff41cc5247197a6caf50216c4c76cc29d78a97d"
	date := &metav1.Time{Time: metav1.Now().Time}
	revisionAuthor := "test test@test.com"
	references := make([]appv1.RevisionReference, 0)
	revReference := appv1.RevisionReference{
		Commit: &appv1.CommitMetadata{
			Author:  "testAuthor",
			Subject: "test",
			RepoURL: repoURL,
			SHA:     "3ff41cc5247197a6caf50216c4c76cc29d78a97c",
		},
	}
	references = append(references, revReference)
	hydratedCommitMetadata := HydratorCommitMetadata{
		RepoURL:    repoURL,
		DrySHA:     drySHA,
		Author:     revisionAuthor,
		Date:       date.Format(time.RFC3339),
		References: references,
		Subject:    "testMessage",
	}
	type args struct {
		repoURL           string
		drySha            string
		dryCommitMetadata *appv1.RevisionMetadata
	}
	tests := []struct {
		name    string
		args    args
		want    HydratorCommitMetadata
		wantErr bool
	}{
		{
			name: "test GetHydratorCommitMD",
			args: args{
				repoURL: repoURL,
				drySha:  drySHA,
				dryCommitMetadata: &appv1.RevisionMetadata{
					Author:     revisionAuthor,
					Date:       date,
					Message:    "testMessage",
					References: references,
				},
			},
			want: hydratedCommitMetadata,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetCommitMetadata(tt.args.repoURL, tt.args.drySha, tt.args.dryCommitMetadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCommitMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCommitMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}
