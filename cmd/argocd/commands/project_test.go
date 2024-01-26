package commands

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newProj(name string) *v1alpha1.AppProject {
	return &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Server:    "*",
					Namespace: "*",
				},
			},
		},
	}
}

func TestPrintProjectTable(t *testing.T) {
	output, err := captureOutput(func() error {
		baseProj := newProj("base")
		projSpecAllowedNodeLabels := baseProj.DeepCopy()
		projSpecAllowedNodeLabels.ObjectMeta.Name = "allowed-labels"
		projSpecAllowedNodeLabels.Spec.AllowedNodeLabels = []string{
			"foo",
			"bar",
		}
		printProjectTable([]v1alpha1.AppProject{*baseProj, *projSpecAllowedNodeLabels})
		return nil
	})
	assert.NoError(t, err)
	expectation := `NAME            DESCRIPTION  DESTINATIONS  SOURCES  CLUSTER-RESOURCE-WHITELIST  NAMESPACE-RESOURCE-BLACKLIST  SIGNATURE-KEYS  ORPHANED-RESOURCES  ALLOWED-NODE-LABELS
base                         *,*           *        <none>                      <none>                        <none>          disabled            <none>
allowed-labels               *,*           *        <none>                      <none>                        <none>          disabled            2 label(s)
`
	assert.Equal(t, expectation, output)
}

func TestGetProjectSpec(t *testing.T) {
	baseProj := newProj("base")
	projSpecAllowedNodeLabels := baseProj.DeepCopy()
	projSpecAllowedNodeLabels.Spec.AllowedNodeLabels = []string{
		"foo",
		"bar",
	}

	for _, tt := range []struct {
		name           string
		proj           *v1alpha1.AppProject
		expectedOutput string
	}{
		{
			name: "project with default spec",
			proj: baseProj,
			expectedOutput: `destinations:
- namespace: '*'
  server: '*'
sourceRepos:
- '*'
`,
		},
		{
			name: "project spec with allowed node labels",
			proj: projSpecAllowedNodeLabels,
			expectedOutput: `allowedNodeLabels:
- foo
- bar
destinations:
- namespace: '*'
  server: '*'
sourceRepos:
- '*'
`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureOutput(func() error {
				err := PrintResource(tt.proj.Spec, "yaml")
				return err
			})
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, out)
		})
	}

}
