package hydrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name     string
		metadata HydratorCommitMetadata
		want     string
		wantErr  bool
	}{
		{
			name: "author and multiple references",
			metadata: HydratorCommitMetadata{
				RepoURL: "https://github.com/test/argocd-example-apps",
				DrySHA:  "3ff41cc5247197a6caf50216c4c76cc29d78a97d",
				Author:  "test <test@test.com>",
				Date:    metav1.Now().String(),
				References: []v1alpha1.RevisionReference{
					{
						Commit: &v1alpha1.CommitMetadata{
							Author:  "ref test <ref-test@test.com>",
							Subject: "test",
							RepoURL: "https://github.com/test/argocd-example-apps",
							SHA:     "3ff41cc5247197a6caf50216c4c76cc29d78a97c",
						},
					},
					{
						Commit: &v1alpha1.CommitMetadata{
							Author:  "ref test 2 <ref-test-2@test.com>",
							Subject: "test 2",
							RepoURL: "https://github.com/test/argocd-example-apps",
							SHA:     "abc12345678912345678912345678912345678912",
						},
					},
				},
				Body:    "testBody",
				Subject: "testSubject",
			},
			want: `3ff41cc: testSubject

testBody

Co-authored-by: ref test <ref-test@test.com>
Co-authored-by: ref test 2 <ref-test-2@test.com>
Co-authored-by: test <test@test.com>
`,
		},
		{
			name: "no references",
			metadata: HydratorCommitMetadata{
				RepoURL: "https://github.com/test/argocd-example-apps",
				DrySHA:  "3ff41cc5247197a6caf50216c4c76cc29d78a97d",
				Author:  "test <test@test.com>",
				Date:    metav1.Now().String(),
				Body:    "testBody",
				Subject: "testSubject",
			},
			want: `3ff41cc: testSubject

testBody

Co-authored-by: test <test@test.com>
`,
		},
		{
			name: "no body",
			metadata: HydratorCommitMetadata{
				RepoURL: "https://github.com/test/argocd-example-apps",
				DrySHA:  "3ff41cc5247197a6caf50216c4c76cc29d78a97d",
				Author:  "test <test@test.com>",
				Date:    metav1.Now().String(),
				Subject: "testSubject",
			},
			want: `3ff41cc: testSubject

Co-authored-by: test <test@test.com>
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(settings.CommitMessageTemplate, tt.metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
