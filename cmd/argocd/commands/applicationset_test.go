package commands

import (
	"io"
	"os"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPrintApplicationSetNames(t *testing.T) {
	output, _ := captureOutput(func() error {
		appSet := &v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		appSet2 := &v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "team-one",
				Name:      "test",
			},
		}
		printApplicationSetNames([]v1alpha1.ApplicationSet{*appSet, *appSet2})
		return nil
	})
	expectation := "test\nteam-one/test\n"
	if output != expectation {
		t.Fatalf("Incorrect print params output %q, should be %q", output, expectation)
	}
}

func TestPrintApplicationSetTable(t *testing.T) {
	output, err := captureOutput(func() error {
		app := &v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Git: &v1alpha1.GitGenerator{
							RepoURL:  "https://github.com/argoproj/argo-cd.git",
							Revision: "head",
							Directories: []v1alpha1.GitGeneratorItem{
								{
									Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
								},
							},
						},
					},
				},
				Template: v1alpha1.ApplicationSetTemplate{
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
					},
				},
			},
			Status: v1alpha1.ApplicationSetStatus{
				Conditions: []v1alpha1.ApplicationSetCondition{
					{
						Status: v1alpha1.ApplicationSetConditionStatusTrue,
						Type:   v1alpha1.ApplicationSetConditionResourcesUpToDate,
					},
				},
			},
		}

		app2 := &v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app-name",
				Namespace: "team-two",
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						Git: &v1alpha1.GitGenerator{
							RepoURL:  "https://github.com/argoproj/argo-cd.git",
							Revision: "head",
							Directories: []v1alpha1.GitGeneratorItem{
								{
									Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
								},
							},
						},
					},
				},
				Template: v1alpha1.ApplicationSetTemplate{
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
					},
				},
			},
			Status: v1alpha1.ApplicationSetStatus{
				Conditions: []v1alpha1.ApplicationSetCondition{
					{
						Status: v1alpha1.ApplicationSetConditionStatusTrue,
						Type:   v1alpha1.ApplicationSetConditionResourcesUpToDate,
					},
				},
			},
		}
		output := "table"
		printApplicationSetTable([]v1alpha1.ApplicationSet{*app, *app2}, &output)
		return nil
	})
	assert.NoError(t, err)
	expectation := "NAME               PROJECT  SYNCPOLICY  CONDITIONS\napp-name           default  nil         [{ResourcesUpToDate  <nil> True }]\nteam-two/app-name  default  nil         [{ResourcesUpToDate  <nil> True }]\n"
	assert.Equal(t, expectation, output)
}

func TestPrintAppSetSummaryTable(t *testing.T) {
	baseAppSet := &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-name",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Git: &v1alpha1.GitGenerator{
						RepoURL:  "https://github.com/argoproj/argo-cd.git",
						Revision: "head",
						Directories: []v1alpha1.GitGeneratorItem{
							{
								Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
							},
						},
					},
				},
			},
			Template: v1alpha1.ApplicationSetTemplate{
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
				},
			},
		},
		Status: v1alpha1.ApplicationSetStatus{
			Conditions: []v1alpha1.ApplicationSetCondition{
				{
					Status: v1alpha1.ApplicationSetConditionStatusTrue,
					Type:   v1alpha1.ApplicationSetConditionResourcesUpToDate,
				},
			},
		},
	}

	appsetSpecSyncPolicy := baseAppSet.DeepCopy()
	appsetSpecSyncPolicy.Spec.SyncPolicy = &v1alpha1.ApplicationSetSyncPolicy{
		PreserveResourcesOnDeletion: true,
	}

	appSetTemplateSpecSyncPolicy := baseAppSet.DeepCopy()
	appSetTemplateSpecSyncPolicy.Spec.Template.Spec.SyncPolicy = &v1alpha1.SyncPolicy{
		Automated: &v1alpha1.SyncPolicyAutomated{
			SelfHeal: true,
		},
	}

	appSetBothSyncPolicies := baseAppSet.DeepCopy()
	appSetBothSyncPolicies.Spec.SyncPolicy = &v1alpha1.ApplicationSetSyncPolicy{
		PreserveResourcesOnDeletion: true,
	}
	appSetBothSyncPolicies.Spec.Template.Spec.SyncPolicy = &v1alpha1.SyncPolicy{
		Automated: &v1alpha1.SyncPolicyAutomated{
			SelfHeal: true,
		},
	}

	for _, tt := range []struct {
		name           string
		appSet         *v1alpha1.ApplicationSet
		expectedOutput string
	}{
		{
			name:   "appset with only spec.syncPolicy set",
			appSet: appsetSpecSyncPolicy,
			expectedOutput: `Name:               app-name
Project:            default
Server:             
Namespace:          
Repo:               
Target:             
Path:               
SyncPolicy:         <none>
`,
		},
		{
			name:   "appset with only spec.template.spec.syncPolicy set",
			appSet: appSetTemplateSpecSyncPolicy,
			expectedOutput: `Name:               app-name
Project:            default
Server:             
Namespace:          
Repo:               
Target:             
Path:               
SyncPolicy:         Automated
`,
		},
		{
			name:   "appset with both spec.SyncPolicy and spec.template.spec.syncPolicy set",
			appSet: appSetBothSyncPolicies,
			expectedOutput: `Name:               app-name
Project:            default
Server:             
Namespace:          
Repo:               
Target:             
Path:               
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

			printAppSetSummaryTable(tt.appSet)
			w.Close()

			out, err := io.ReadAll(r)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, string(out))
		})
	}
}
