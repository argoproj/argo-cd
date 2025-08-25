package hydrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var commitMessageTemplate = `{{- if .metadata }}
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

func TestRender(t *testing.T) {
	type args struct {
		tmpl string
		data HydratorCommitMetadata
	}

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

	commitMetadata := HydratorCommitMetadata{
		RepoURL:    "https://github.com/test/argocd-example-apps",
		DrySHA:     "3ff41cc5247197a6caf50216c4c76cc29d78a97d",
		Author:     "test test@test.com",
		Date:       metav1.Now().String(),
		References: references,
		Body:       "testBody",
		Subject:    "testSubject",
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test render",
			args: args{
				tmpl: commitMessageTemplate,
				data: commitMetadata,
			},
		},
		{
			name: "test render with empty template",
			args: args{
				data: commitMetadata,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.args.tmpl, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.args.tmpl != "" {
				assert.NotEmpty(t, got)
				assert.Contains(t, got, "Commit")
			} else {
				assert.Empty(t, got)
			}
		})
	}
}
