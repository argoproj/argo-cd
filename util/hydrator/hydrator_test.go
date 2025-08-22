package hydrator

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestGetCommitMetadata(t *testing.T) {
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
