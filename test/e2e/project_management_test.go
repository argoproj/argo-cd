package e2e

import (
	"strconv"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProjectManagement(t *testing.T) {
	t.Run("TestProjectCreation", func(t *testing.T) {
		projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
		_, err := fixture.RunCli("proj", "create",
			"--name", projectName,
			"--description", "Test description",
			"-d", "https://192.168.99.100:8443,default",
			"-d", "https://192.168.99.100:8443,service")
		if err != nil {
			t.Fatalf("Unable to create project %v", err)
		}

		proj, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Get(projectName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unable to get project %v", err)
		}
		assert.Equal(t, projectName, proj.Name)
		assert.Equal(t, 2, len(proj.Spec.Destinations))

		assert.Equal(t, "https://192.168.99.100:8443", proj.Spec.Destinations[0].Server)
		assert.Equal(t, "default", proj.Spec.Destinations[0].Namespace)

		assert.Equal(t, "https://192.168.99.100:8443", proj.Spec.Destinations[1].Server)
		assert.Equal(t, "service", proj.Spec.Destinations[1].Namespace)
	})

	t.Run("TestProjectDeletion", func(t *testing.T) {
		projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
		_, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Create(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}})
		if err != nil {
			t.Fatalf("Unable to create project %v", err)
		}

		_, err = fixture.RunCli("proj", "rm", projectName)
		if err != nil {
			t.Fatalf("Unable to delete project %v", err)
		}

		_, err = fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Get(projectName, metav1.GetOptions{})
		assert.True(t, errors.IsNotFound(err))
	})
}
