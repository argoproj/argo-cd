package e2e

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

func TestProjectManagement(t *testing.T) {
	assertProjHasEvent := func(a *v1alpha1.AppProject, action string, reason string) {
		list, err := fixture.KubeClient.CoreV1().Events(fixture.Namespace).List(metav1.ListOptions{
			FieldSelector: fields.SelectorFromSet(map[string]string{
				"involvedObject.name":      a.Name,
				"involvedObject.uid":       string(a.UID),
				"involvedObject.namespace": fixture.Namespace,
			}).String(),
		})
		if err != nil {
			t.Fatalf("Unable to get app events %v", err)
		}
		for i := range list.Items {
			event := list.Items[i]
			if event.Reason == reason && event.Action == action {
				return
			}
		}
		t.Errorf("Unable to find event with reason=%s; action=%s", reason, action)
	}

	t.Run("TestProjectCreation", func(t *testing.T) {
		projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
		_, err := fixture.RunCli("proj", "create", projectName,
			"--description", "Test description",
			"-d", "https://192.168.99.100:8443,default",
			"-d", "https://192.168.99.100:8443,service",
			"-s", "https://github.com/argoproj/argo-cd.git")
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

		assert.Equal(t, 1, len(proj.Spec.SourceRepos))
		assert.Equal(t, "https://github.com/argoproj/argo-cd.git", proj.Spec.SourceRepos[0])

		assertProjHasEvent(proj, "create", argo.EventReasonResourceCreated)
	})

	t.Run("TestProjectDeletion", func(t *testing.T) {
		projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
		proj, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Create(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}})
		if err != nil {
			t.Fatalf("Unable to create project %v", err)
		}

		_, err = fixture.RunCli("proj", "delete", projectName)
		if err != nil {
			t.Fatalf("Unable to delete project %v", err)
		}

		_, err = fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Get(projectName, metav1.GetOptions{})
		assert.True(t, errors.IsNotFound(err))
		assertProjHasEvent(proj, "delete", argo.EventReasonResourceDeleted)
	})

	t.Run("TestSetProject", func(t *testing.T) {
		projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
		_, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Create(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}})
		if err != nil {
			t.Fatalf("Unable to create project %v", err)
		}

		_, err = fixture.RunCli("proj", "set", projectName,
			"--description", "updated description",
			"-d", "https://192.168.99.100:8443,default",
			"-d", "https://192.168.99.100:8443,service")
		if err != nil {
			t.Fatalf("Unable to update project %v", err)
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
		assertProjHasEvent(proj, "update", argo.EventReasonResourceUpdated)
	})

	t.Run("TestAddProjectDestination", func(t *testing.T) {
		projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
		_, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Create(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}})
		if err != nil {
			t.Fatalf("Unable to create project %v", err)
		}

		_, err = fixture.RunCli("proj", "add-destination", projectName,
			"https://192.168.99.100:8443",
			"test1",
		)

		if err != nil {
			t.Fatalf("Unable to add project destination %v", err)
		}

		_, err = fixture.RunCli("proj", "add-destination", projectName,
			"https://192.168.99.100:8443",
			"test1",
		)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "already defined"))

		proj, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Get(projectName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unable to get project %v", err)
		}
		assert.Equal(t, projectName, proj.Name)
		assert.Equal(t, 1, len(proj.Spec.Destinations))

		assert.Equal(t, "https://192.168.99.100:8443", proj.Spec.Destinations[0].Server)
		assert.Equal(t, "test1", proj.Spec.Destinations[0].Namespace)
		assertProjHasEvent(proj, "update", argo.EventReasonResourceUpdated)
	})

	t.Run("TestRemoveProjectDestination", func(t *testing.T) {
		projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
		_, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Create(&v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: projectName},
			Spec: v1alpha1.AppProjectSpec{
				Destinations: []v1alpha1.ApplicationDestination{{
					Server:    "https://192.168.99.100:8443",
					Namespace: "test",
				}},
			},
		})

		if err != nil {
			t.Fatalf("Unable to create project %v", err)
		}

		_, err = fixture.RunCli("proj", "remove-destination", projectName,
			"https://192.168.99.100:8443",
			"test",
		)

		if err != nil {
			t.Fatalf("Unable to remove project destination %v", err)
		}

		_, err = fixture.RunCli("proj", "remove-destination", projectName,
			"https://192.168.99.100:8443",
			"test1",
		)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "does not exist"))

		proj, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Get(projectName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unable to get project %v", err)
		}
		assert.Equal(t, projectName, proj.Name)
		assert.Equal(t, 0, len(proj.Spec.Destinations))
		assertProjHasEvent(proj, "update", argo.EventReasonResourceUpdated)
	})

	t.Run("TestAddProjectSource", func(t *testing.T) {
		projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
		_, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Create(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}})
		if err != nil {
			t.Fatalf("Unable to create project %v", err)
		}

		_, err = fixture.RunCli("proj", "add-source", projectName, "https://github.com/argoproj/argo-cd.git")

		if err != nil {
			t.Fatalf("Unable to add project source %v", err)
		}

		_, err = fixture.RunCli("proj", "add-source", projectName, "https://github.com/argoproj/argo-cd.git")
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "already defined"))

		proj, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Get(projectName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unable to get project %v", err)
		}
		assert.Equal(t, projectName, proj.Name)
		assert.Equal(t, 1, len(proj.Spec.SourceRepos))

		assert.Equal(t, "https://github.com/argoproj/argo-cd.git", proj.Spec.SourceRepos[0])
	})

	t.Run("TestRemoveProjectSource", func(t *testing.T) {
		projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
		_, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Create(&v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: projectName},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos: []string{"https://github.com/argoproj/argo-cd.git"},
			},
		})

		if err != nil {
			t.Fatalf("Unable to create project %v", err)
		}

		_, err = fixture.RunCli("proj", "remove-source", projectName, "https://github.com/argoproj/argo-cd.git")

		if err != nil {
			t.Fatalf("Unable to remove project source %v", err)
		}

		_, err = fixture.RunCli("proj", "remove-source", projectName, "https://github.com/argoproj/argo-cd.git")
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "does not exist"))

		proj, err := fixture.AppClient.ArgoprojV1alpha1().AppProjects(fixture.Namespace).Get(projectName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unable to get project %v", err)
		}
		assert.Equal(t, projectName, proj.Name)
		assert.Equal(t, 0, len(proj.Spec.SourceRepos))
		assertProjHasEvent(proj, "update", argo.EventReasonResourceUpdated)
	})
}
