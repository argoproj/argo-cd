package commands

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	arogappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
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
				Template: arogappsetv1.ApplicationSetTemplate{
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
					},
				},
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
		output := "table"
		printApplicationSetTable([]arogappsetv1.ApplicationSet{*app, *app}, &output)
		return nil
	})
	assert.NoError(t, err)
	expectation := "NAME      NAMESPACE  PROJECT  SYNCPOLICY  CONDITIONS\napp-name             default  nil         [{ResourcesUpToDate  <nil> True }]\napp-name             default  nil         [{ResourcesUpToDate  <nil> True }]\n"
	assert.Equal(t, expectation, output)
}
