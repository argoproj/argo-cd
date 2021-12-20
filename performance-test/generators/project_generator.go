package generator

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
)

type ProjectGenerator struct {
	clientSet *appclientset.Clientset
}

func NewProjectGenerator(clientSet *appclientset.Clientset) Generator {
	return &ProjectGenerator{clientSet}
}

func (pg *ProjectGenerator) Generate() error {
	projects := pg.clientSet.ArgoprojV1alpha1().AppProjects("argocd")
	_, err := projects.Create(context.TODO(), &v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "project-",
			Namespace:    "argocd",
		},
		Spec: v1alpha1.AppProjectSpec{
			Description: "generated-project",
		},
	}, v1.CreateOptions{})
	return err
}
