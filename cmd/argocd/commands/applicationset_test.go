package commands

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	testutils "github.com/argoproj/argo-cd/v2/test"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	arogappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPrintApplicationSetNames(t *testing.T) {
	output, _ := captureOutput(func() error {
		appSet := &arogappsetv1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		printApplicationSetNames([]arogappsetv1.ApplicationSet{*appSet, *appSet})
		return nil
	})
	expectation := "test\ntest\n"
	if output != expectation {
		t.Fatalf("Incorrect print params output %q, should be %q", output, expectation)
	}
}

func TestPrintApplicationSetTable(t *testing.T) {
	output, err := captureOutput(func() error {
		app := &arogappsetv1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name",
			},
			Spec: arogappsetv1.ApplicationSetSpec{
				Generators: []arogappsetv1.ApplicationSetGenerator{
					{
						Git: &arogappsetv1.GitGenerator{
							RepoURL:  "https://github.com/argoproj/argo-cd.git",
							Revision: "head",
							Directories: []arogappsetv1.GitDirectoryGeneratorItem{
								{
									Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
								},
							},
						},
					},
				},
				Template: apiextensionsv1.JSON{Raw: []byte(`{"spec":{"project": "default"}}`)},
			},
			Status: arogappsetv1.ApplicationSetStatus{
				Conditions: []arogappsetv1.ApplicationSetCondition{
					{
						Status: v1alpha1.ApplicationSetConditionStatusTrue,
						Type:   arogappsetv1.ApplicationSetConditionResourcesUpToDate,
					},
				},
			},
		}
		output := "table"
		printApplicationSetTable([]arogappsetv1.ApplicationSet{*app, *app}, &output)
		return nil
	})
	assert.NoError(t, err)
	expectation := `NAME      NAMESPACE  PROJECT  SYNCPOLICY  CONDITIONS
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]
`
	assert.Equal(t, expectation, output)
}

func TestPrintApplicationSetTableWithTemplatedFields(t *testing.T) {
	output, err := captureOutput(func() error {
		app := &arogappsetv1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name",
			},
			Spec: arogappsetv1.ApplicationSetSpec{
				Generators: []arogappsetv1.ApplicationSetGenerator{
					arogappsetv1.ApplicationSetGenerator{
						Git: &arogappsetv1.GitGenerator{
							RepoURL:  "https://github.com/argoproj/argo-cd.git",
							Revision: "head",
							Directories: []arogappsetv1.GitDirectoryGeneratorItem{
								arogappsetv1.GitDirectoryGeneratorItem{
									Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
								},
							},
						},
					},
				},
				Template: apiextensionsv1.JSON{Raw: []byte(`{"spec":{"project": "default", "source": {"repoURL": "https://github.com/argoproj/argocd-example-apps", "targetRevision": "{{ .targetRevision }}", "path": "guestbook"}}}`)},
			},
			Status: arogappsetv1.ApplicationSetStatus{
				Conditions: []arogappsetv1.ApplicationSetCondition{
					arogappsetv1.ApplicationSetCondition{
						Status: v1alpha1.ApplicationSetConditionStatusTrue,
						Type:   arogappsetv1.ApplicationSetConditionResourcesUpToDate,
					},
				},
			},
		}
		output := "wide"
		printApplicationSetTable([]arogappsetv1.ApplicationSet{*app, *app}, &output)
		return nil
	})
	assert.NoError(t, err)
	expectation := `NAME      NAMESPACE  PROJECT  SYNCPOLICY  CONDITIONS                          REPO                                             PATH       TARGET
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]  https://github.com/argoproj/argocd-example-apps  guestbook  {{ .targetRevision }}
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]  https://github.com/argoproj/argocd-example-apps  guestbook  {{ .targetRevision }}
`
	assert.Equal(t, expectation, output)
}

func TestSourceTemplated(t *testing.T) {
	output, err := captureOutput(func() error {
		app := &arogappsetv1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name",
			},
			Spec: arogappsetv1.ApplicationSetSpec{
				Generators: []arogappsetv1.ApplicationSetGenerator{
					arogappsetv1.ApplicationSetGenerator{
						Git: &arogappsetv1.GitGenerator{
							RepoURL:  "https://github.com/argoproj/argo-cd.git",
							Revision: "head",
							Directories: []arogappsetv1.GitDirectoryGeneratorItem{
								arogappsetv1.GitDirectoryGeneratorItem{
									Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
								},
							},
						},
					},
				},
				Template: apiextensionsv1.JSON{Raw: []byte(`{"spec":{"project": "default", 
					"{{ ternary \"source\" \"nosourcegit\" (eq .type \"git\") }}": {"repoURL": "https://github.com/argoproj/argocd-example-apps", "targetRevision": "{{ .targetRevision }}", "path": "guestbook"}, 
					"{{ ternary \"source\" \"nosourcehelm\" (eq .type \"helm\") }}": {"repoURL": "https://chart.github.com/argoproj/argocd-example-apps", "targetRevision": "{{ .targetRevision }}", "path": "guestbook"}}}`)},
			},
			Status: arogappsetv1.ApplicationSetStatus{
				Conditions: []arogappsetv1.ApplicationSetCondition{
					arogappsetv1.ApplicationSetCondition{
						Status: v1alpha1.ApplicationSetConditionStatusTrue,
						Type:   arogappsetv1.ApplicationSetConditionResourcesUpToDate,
					},
				},
			},
		}
		output := "wide"
		printApplicationSetTable([]arogappsetv1.ApplicationSet{*app, *app}, &output)
		return nil
	})
	assert.NoError(t, err)
	expectation := `NAME      NAMESPACE  PROJECT  SYNCPOLICY  CONDITIONS                          REPO       PATH       TARGET
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]  Templated  Templated  Templated
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]  Templated  Templated  Templated
`
	assert.Equal(t, expectation, output)
}

func TestShowAppSetTemplate(t *testing.T) {
	output, _ := captureOutput(func() error {

		template := map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":   "{{ .name }}",
				"labels": map[string]string{"foo": "{{ .bar }}"},
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]string{
					"repoURL":        "https://github.com/argoproj/argocd-example-apps",
					"targetRevision": "{{ .targetRevision }}",
					"path":           "guestbook",
				},
			},
		}

		showAppSetTemplate(template)
		return nil
	})
	expectation := `Template:
metadata:
  labels:
    foo: '{{ .bar }}'
  name: '{{ .name }}'
spec:
  project: default
  source:
    path: guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps
    targetRevision: '{{ .targetRevision }}'
`
	assert.Equal(t, expectation, output)
}

func TestPrintAppSetSummaryTable(t *testing.T) {
	baseAppSet := &arogappsetv1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-name",
		},
		Spec: arogappsetv1.ApplicationSetSpec{
			Generators: []arogappsetv1.ApplicationSetGenerator{
				arogappsetv1.ApplicationSetGenerator{
					Git: &arogappsetv1.GitGenerator{
						RepoURL:  "https://github.com/argoproj/argo-cd.git",
						Revision: "head",
						Directories: []arogappsetv1.GitDirectoryGeneratorItem{
							arogappsetv1.GitDirectoryGeneratorItem{
								Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
							},
						},
					},
				},
			},
			Template: apiextensionsv1.JSON{Raw: []byte(`{"spec":{"project": "default"}}`)},
		},
		Status: arogappsetv1.ApplicationSetStatus{
			Conditions: []arogappsetv1.ApplicationSetCondition{
				arogappsetv1.ApplicationSetCondition{
					Status: v1alpha1.ApplicationSetConditionStatusTrue,
					Type:   arogappsetv1.ApplicationSetConditionResourcesUpToDate,
				},
			},
		},
	}

	appsetSpecSyncPolicy := baseAppSet.DeepCopy()

	appsetSpecSyncPolicy.Spec.SyncPolicy = &arogappsetv1.ApplicationSetSyncPolicy{
		PreserveResourcesOnDeletion: true,
	}

	appSetTemplateSpecSyncPolicy := baseAppSet.DeepCopy()

	appSetTemplateSpecSyncPolicy.Spec.Template = testutils.UpdateDataAsJson(appSetTemplateSpecSyncPolicy.Spec.Template, "/spec/syncPolicy", map[string]interface{}{"automated": map[string]interface{}{"selfHeal": true}})

	appSetBothSyncPolicies := baseAppSet.DeepCopy()
	appSetBothSyncPolicies.Spec.SyncPolicy = &arogappsetv1.ApplicationSetSyncPolicy{
		PreserveResourcesOnDeletion: true,
	}
	appSetBothSyncPolicies.Spec.Template = testutils.UpdateDataAsJson(appSetTemplateSpecSyncPolicy.Spec.Template, "/spec/syncPolicy", map[string]interface{}{"automated": map[string]interface{}{"selfHeal": true}})

	for _, tt := range []struct {
		name           string
		appSet         *arogappsetv1.ApplicationSet
		expectedOutput string
	}{
		{
			name:   "appset with only spec.syncPolicy set",
			appSet: appsetSpecSyncPolicy,
			expectedOutput: `Name:               app-name
Project:            default
SyncPolicy:         <none>
`,
		},
		{
			name:   "appset with only spec.template.spec.syncPolicy set",
			appSet: appSetTemplateSpecSyncPolicy,
			expectedOutput: `Name:               app-name
Project:            default
SyncPolicy:         Automated
`,
		},
		{
			name:   "appset with both spec.SyncPolicy and spec.template.spec.syncPolicy set",
			appSet: appSetBothSyncPolicies,
			expectedOutput: `Name:               app-name
Project:            default
SyncPolicy:         Automated
`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			oldStdout := os.Stdout
			defer func() {
				os.Stdout = oldStdout
			}()

			r, w, _ := os.Pipe()
			os.Stdout = w

			var template map[string]interface{}
			err := json.Unmarshal(tt.appSet.Spec.Template.Raw, &template)
			errors.CheckError(err)

			printAppSetSummaryTable(tt.appSet, template)
			w.Close()

			out, err := io.ReadAll(r)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, string(out))
		})
	}
}
