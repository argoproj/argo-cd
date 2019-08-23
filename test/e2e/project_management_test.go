package e2e

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/util/argo"
)

func assertProjHasEvent(t *testing.T, a *v1alpha1.AppProject, message string, reason string) {
	list, err := fixture.KubeClientset.CoreV1().Events(fixture.ArgoCDNamespace).List(metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(map[string]string{
			"involvedObject.name":      a.Name,
			"involvedObject.uid":       string(a.UID),
			"involvedObject.namespace": fixture.ArgoCDNamespace,
		}).String(),
	})
	assert.NoError(t, err)

	for i := range list.Items {
		event := list.Items[i]
		if event.Reason == reason && strings.Contains(event.Message, message) {
			return
		}
	}
	t.Errorf("Unable to find event with reason=%s; message=%s", reason, message)
}

func TestProjectCreation(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + fixture.Name()
	_, err := fixture.RunCli("proj", "create", projectName,
		"--description", "Test description",
		"-d", "https://192.168.99.100:8443,default",
		"-d", "https://192.168.99.100:8443,service",
		"-s", "https://github.com/argoproj/argo-cd.git",
		"--orphaned-resources")
	assert.Nil(t, err)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, projectName, proj.Name)
	assert.Equal(t, 2, len(proj.Spec.Destinations))

	assert.Equal(t, "https://192.168.99.100:8443", proj.Spec.Destinations[0].Server)
	assert.Equal(t, "default", proj.Spec.Destinations[0].Namespace)

	assert.Equal(t, "https://192.168.99.100:8443", proj.Spec.Destinations[1].Server)
	assert.Equal(t, "service", proj.Spec.Destinations[1].Namespace)

	assert.Equal(t, 1, len(proj.Spec.SourceRepos))
	assert.Equal(t, "https://github.com/argoproj/argo-cd.git", proj.Spec.SourceRepos[0])

	assert.NotNil(t, proj.Spec.OrphanedResources)
	assert.True(t, proj.Spec.OrphanedResources.IsWarn())

	assertProjHasEvent(t, proj, "create", argo.EventReasonResourceCreated)
}

func TestProjectDeletion(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}})
	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "delete", projectName)
	assert.NoError(t, err)

	_, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(projectName, metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))
	assertProjHasEvent(t, proj, "delete", argo.EventReasonResourceDeleted)
}

func TestSetProject(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}})
	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "set", projectName,
		"--description", "updated description",
		"-d", "https://192.168.99.100:8443,default",
		"-d", "https://192.168.99.100:8443,service",
		"--orphaned-resources-warn=false")
	assert.NoError(t, err)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, projectName, proj.Name)
	assert.Equal(t, 2, len(proj.Spec.Destinations))

	assert.Equal(t, "https://192.168.99.100:8443", proj.Spec.Destinations[0].Server)
	assert.Equal(t, "default", proj.Spec.Destinations[0].Namespace)

	assert.Equal(t, "https://192.168.99.100:8443", proj.Spec.Destinations[1].Server)
	assert.Equal(t, "service", proj.Spec.Destinations[1].Namespace)

	assert.NotNil(t, proj.Spec.OrphanedResources)
	assert.False(t, proj.Spec.OrphanedResources.IsWarn())

	assertProjHasEvent(t, proj, "update", argo.EventReasonResourceUpdated)
}

func TestAddProjectDestination(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}})
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
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "already defined"))

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, projectName, proj.Name)
	assert.Equal(t, 1, len(proj.Spec.Destinations))

	assert.Equal(t, "https://192.168.99.100:8443", proj.Spec.Destinations[0].Server)
	assert.Equal(t, "test1", proj.Spec.Destinations[0].Namespace)
	assertProjHasEvent(t, proj, "update", argo.EventReasonResourceUpdated)
}

func TestRemoveProjectDestination(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(&v1alpha1.AppProject{
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(projectName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unable to get project %v", err)
	}
	assert.Equal(t, projectName, proj.Name)
	assert.Equal(t, 0, len(proj.Spec.Destinations))
	assertProjHasEvent(t, proj, "update", argo.EventReasonResourceUpdated)
}

func TestAddProjectSource(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}})
	if err != nil {
		t.Fatalf("Unable to create project %v", err)
	}

	_, err = fixture.RunCli("proj", "add-source", projectName, "https://github.com/argoproj/argo-cd.git")

	if err != nil {
		t.Fatalf("Unable to add project source %v", err)
	}

	_, err = fixture.RunCli("proj", "add-source", projectName, "https://github.com/argoproj/argo-cd.git")
	assert.Nil(t, err)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, projectName, proj.Name)
	assert.Equal(t, 1, len(proj.Spec.SourceRepos))

	assert.Equal(t, "https://github.com/argoproj/argo-cd.git", proj.Spec.SourceRepos[0])
}

func TestRemoveProjectSource(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(&v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projectName},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"https://github.com/argoproj/argo-cd.git"},
		},
	})

	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "remove-source", projectName, "https://github.com/argoproj/argo-cd.git")

	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "remove-source", projectName, "https://github.com/argoproj/argo-cd.git")
	assert.NoError(t, err)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, projectName, proj.Name)
	assert.Equal(t, 0, len(proj.Spec.SourceRepos))
	assertProjHasEvent(t, proj, "update", argo.EventReasonResourceUpdated)
}

func TestUseJWTToken(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	appName := "app-" + strconv.FormatInt(time.Now().Unix(), 10)
	roleName := "roleTest"
	testApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: appName,
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: v1alpha1.ApplicationSource{
				RepoURL: fixture.RepoURL(fixture.RepoURLTypeFile),
				Path:    "guestbook",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    common.KubernetesInternalAPIServerAddr,
				Namespace: fixture.ArgoCDNamespace,
			},
			Project: projectName,
		},
	}
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(&v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projectName},
		Spec: v1alpha1.AppProjectSpec{
			Destinations: []v1alpha1.ApplicationDestination{{
				Server:    common.KubernetesInternalAPIServerAddr,
				Namespace: fixture.ArgoCDNamespace,
			}},
			SourceRepos: []string{"*"},
		},
	})
	assert.Nil(t, err)

	_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(testApp)
	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "role", "create", projectName, roleName)
	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "role", "create-token", projectName, roleName)
	assert.NoError(t, err)

	for _, action := range []string{"get", "update", "sync", "create", "override", "*"} {
		_, err = fixture.RunCli("proj", "role", "add-policy", projectName, roleName, "-a", action, "-o", "*", "-p", "allow")
		assert.NoError(t, err)
	}
}
