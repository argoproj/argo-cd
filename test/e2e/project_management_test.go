package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/util/argo"
)

func assertProjHasEvent(t *testing.T, a *v1alpha1.AppProject, message string, reason string) {
	list, err := fixture.KubeClientset.CoreV1().Events(fixture.ArgoCDNamespace).List(context.Background(), metav1.ListOptions{
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

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
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

	// create a manifest with the same name to upsert
	newDescription := "Upserted description"
	proj.Spec.Description = newDescription
	proj.ResourceVersion = ""
	data, err := json.Marshal(proj)
	stdinString := string(data)
	assert.NoError(t, err)

	// fail without upsert flag
	_, err = fixture.RunCliWithStdin(stdinString, "proj", "create",
		"-f", "-")
	assert.Error(t, err)

	// succeed with the upsert flag
	_, err = fixture.RunCliWithStdin(stdinString, "proj", "create",
		"-f", "-", "--upsert")
	assert.NoError(t, err)
	proj, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, newDescription, proj.Spec.Description)
}

func TestProjectDeletion(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(
		context.Background(), &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}}, metav1.CreateOptions{})
	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "delete", projectName)
	assert.NoError(t, err)

	_, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))
	assertProjHasEvent(t, proj, "delete", argo.EventReasonResourceDeleted)
}

func TestSetProject(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(
		context.Background(), &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}}, metav1.CreateOptions{})
	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "set", projectName,
		"--description", "updated description",
		"-d", "https://192.168.99.100:8443,default",
		"-d", "https://192.168.99.100:8443,service",
		"--orphaned-resources-warn=false")
	assert.NoError(t, err)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
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
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(
		context.Background(), &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}}, metav1.CreateOptions{})
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

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
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
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(context.Background(), &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projectName},
		Spec: v1alpha1.AppProjectSpec{
			Destinations: []v1alpha1.ApplicationDestination{{
				Server:    "https://192.168.99.100:8443",
				Namespace: "test",
			}},
		},
	}, metav1.CreateOptions{})

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

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
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
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(
		context.Background(), &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Unable to create project %v", err)
	}

	_, err = fixture.RunCli("proj", "add-source", projectName, "https://github.com/argoproj/argo-cd.git")

	if err != nil {
		t.Fatalf("Unable to add project source %v", err)
	}

	_, err = fixture.RunCli("proj", "add-source", projectName, "https://github.com/argoproj/argo-cd.git")
	assert.Nil(t, err)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, projectName, proj.Name)
	assert.Equal(t, 1, len(proj.Spec.SourceRepos))

	assert.Equal(t, "https://github.com/argoproj/argo-cd.git", proj.Spec.SourceRepos[0])
}

func TestRemoveProjectSource(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(context.Background(), &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projectName},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"https://github.com/argoproj/argo-cd.git"},
		},
	}, metav1.CreateOptions{})

	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "remove-source", projectName, "https://github.com/argoproj/argo-cd.git")

	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "remove-source", projectName, "https://github.com/argoproj/argo-cd.git")
	assert.NoError(t, err)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
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
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(context.Background(), &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projectName},
		Spec: v1alpha1.AppProjectSpec{
			Destinations: []v1alpha1.ApplicationDestination{{
				Server:    common.KubernetesInternalAPIServerAddr,
				Namespace: fixture.ArgoCDNamespace,
			}},
			SourceRepos: []string{"*"},
		},
	}, metav1.CreateOptions{})
	assert.Nil(t, err)

	_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(context.Background(), testApp, metav1.CreateOptions{})
	assert.NoError(t, err)

	_, err = fixture.RunCli("proj", "role", "create", projectName, roleName)
	assert.NoError(t, err)

	roleGetResult, err := fixture.RunCli("proj", "role", "get", projectName, roleName)
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(roleGetResult, "ID  ISSUED-AT  EXPIRES-AT"))

	_, err = fixture.RunCli("proj", "role", "create-token", projectName, roleName)
	assert.NoError(t, err)

	for _, action := range []string{"get", "update", "sync", "create", "override", "*"} {
		_, err = fixture.RunCli("proj", "role", "add-policy", projectName, roleName, "-a", action, "-o", "*", "-p", "allow")
		assert.NoError(t, err)
	}

	newProj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Len(t, newProj.Status.JWTTokensByRole[roleName].Items, 1)
	assert.ElementsMatch(t, newProj.Status.JWTTokensByRole[roleName].Items, newProj.Spec.Roles[0].JWTTokens)

	roleGetResult, err = fixture.RunCli("proj", "role", "get", projectName, roleName)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(roleGetResult, strconv.FormatInt((newProj.Status.JWTTokensByRole[roleName].Items[0].IssuedAt), 10)))

	_, err = fixture.RunCli("proj", "role", "delete-token", projectName, roleName, strconv.FormatInt((newProj.Status.JWTTokensByRole[roleName].Items[0].IssuedAt), 10))
	assert.NoError(t, err)
	newProj, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Nil(t, newProj.Status.JWTTokensByRole[roleName].Items)
	assert.Nil(t, newProj.Spec.Roles[0].JWTTokens)

}

func TestAddOrphanedIgnore(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(
		context.Background(), &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: projectName}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Unable to create project %v", err)
	}

	_, err = fixture.RunCli("proj", "add-orphaned-ignore", projectName,
		"group",
		"kind",
		"--name",
		"name",
	)

	if err != nil {
		t.Fatalf("Unable to add resource to orphaned ignore %v", err)
	}

	_, err = fixture.RunCli("proj", "add-orphaned-ignore", projectName,
		"group",
		"kind",
		"--name",
		"name",
	)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "already defined"))

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, projectName, proj.Name)
	assert.Equal(t, 1, len(proj.Spec.OrphanedResources.Ignore))

	assert.Equal(t, "group", proj.Spec.OrphanedResources.Ignore[0].Group)
	assert.Equal(t, "kind", proj.Spec.OrphanedResources.Ignore[0].Kind)
	assert.Equal(t, "name", proj.Spec.OrphanedResources.Ignore[0].Name)
	assertProjHasEvent(t, proj, "update", argo.EventReasonResourceUpdated)
}

func TestRemoveOrphanedIgnore(t *testing.T) {
	fixture.EnsureCleanState(t)

	projectName := "proj-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Create(context.Background(), &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projectName},
		Spec: v1alpha1.AppProjectSpec{
			OrphanedResources: &v1alpha1.OrphanedResourcesMonitorSettings{
				Warn:   pointer.BoolPtr(true),
				Ignore: []v1alpha1.OrphanedResourceKey{{Group: "group", Kind: "kind", Name: "name"}},
			},
		},
	}, metav1.CreateOptions{})

	if err != nil {
		t.Fatalf("Unable to create project %v", err)
	}

	_, err = fixture.RunCli("proj", "remove-orphaned-ignore", projectName,
		"group",
		"kind",
		"--name",
		"name",
	)

	if err != nil {
		t.Fatalf("Unable to remove resource from orphaned ignore list %v", err)
	}

	_, err = fixture.RunCli("proj", "remove-orphaned-ignore", projectName,
		"group",
		"kind",
		"--name",
		"name",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unable to get project %v", err)
	}
	assert.Equal(t, projectName, proj.Name)
	assert.Equal(t, 0, len(proj.Spec.OrphanedResources.Ignore))
	assertProjHasEvent(t, proj, "update", argo.EventReasonResourceUpdated)
}

func createAndConfigGlobalProject() error {
	//Create global project
	projectGlobalName := "proj-g-" + fixture.Name()
	_, err := fixture.RunCli("proj", "create", projectGlobalName,
		"--description", "Test description",
		"-d", "https://192.168.99.100:8443,default",
		"-d", "https://192.168.99.100:8443,service",
		"-s", "https://github.com/argoproj/argo-cd.git",
		"--orphaned-resources")
	if err != nil {
		return err
	}

	projGlobal, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectGlobalName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	projGlobal.Spec.NamespaceResourceBlacklist = []metav1.GroupKind{
		{Group: "", Kind: "Service"},
	}

	projGlobal.Spec.NamespaceResourceWhitelist = []metav1.GroupKind{
		{Group: "", Kind: "Deployment"},
	}

	projGlobal.Spec.ClusterResourceWhitelist = []metav1.GroupKind{
		{Group: "", Kind: "Job"},
	}

	projGlobal.Spec.ClusterResourceBlacklist = []metav1.GroupKind{
		{Group: "", Kind: "Pod"},
	}

	projGlobal.Spec.SyncWindows = v1alpha1.SyncWindows{}
	win := &v1alpha1.SyncWindow{Kind: "deny", Schedule: "* * * * *", Duration: "1h", Applications: []string{"*"}}
	projGlobal.Spec.SyncWindows = append(projGlobal.Spec.SyncWindows, win)

	_, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Update(context.Background(), projGlobal, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	//Configure global project settings
	globalProjectsSettings := `data:
  accounts.config-service: apiKey
  globalProjects: |
    - labelSelector:
        matchExpressions:
          - key: opt
            operator: In
            values:
              - me
              - you
      projectName: %s`

	_, err = fixture.Run("", "kubectl", "patch", "cm", "argocd-cm",
		"-n", fixture.ArgoCDNamespace,
		"-p", fmt.Sprintf(globalProjectsSettings, projGlobal.Name))
	if err != nil {
		return err
	}

	return nil
}

func TestGetVirtualProjectNoMatch(t *testing.T) {
	fixture.EnsureCleanState(t)
	err := createAndConfigGlobalProject()
	assert.NoError(t, err)

	//Create project which does not match global project settings
	projectName := "proj-" + fixture.Name()
	_, err = fixture.RunCli("proj", "create", projectName,
		"--description", "Test description",
		"-d", fmt.Sprintf("%s,*", common.KubernetesInternalAPIServerAddr),
		"-s", "*",
		"--orphaned-resources")
	assert.NoError(t, err)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
	assert.NoError(t, err)

	//Create an app belongs to proj project
	_, err = fixture.RunCli("app", "create", fixture.Name(), "--repo", fixture.RepoURL(fixture.RepoURLTypeFile),
		"--path", guestbookPath, "--project", proj.Name, "--dest-server", common.KubernetesInternalAPIServerAddr, "--dest-namespace", fixture.DeploymentNamespace())
	assert.NoError(t, err)

	//App trying to sync a resource which is not blacked listed anywhere
	_, err = fixture.RunCli("app", "sync", fixture.Name(), "--resource", "apps:Deployment:guestbook-ui", "--timeout", fmt.Sprintf("%v", 10))
	assert.NoError(t, err)

	//app trying to sync a resource which is black listed by global project
	_, err = fixture.RunCli("app", "sync", fixture.Name(), "--resource", ":Service:guestbook-ui", "--timeout", fmt.Sprintf("%v", 10))
	assert.NoError(t, err)

}

func TestGetVirtualProjectMatch(t *testing.T) {
	fixture.EnsureCleanState(t)
	err := createAndConfigGlobalProject()
	assert.NoError(t, err)

	//Create project which matches global project settings
	projectName := "proj-" + fixture.Name()
	_, err = fixture.RunCli("proj", "create", projectName,
		"--description", "Test description",
		"-d", fmt.Sprintf("%s,*", common.KubernetesInternalAPIServerAddr),
		"-s", "*",
		"--orphaned-resources")
	assert.NoError(t, err)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get(context.Background(), projectName, metav1.GetOptions{})
	assert.NoError(t, err)

	//Add a label to this project so that this project match global project selector
	proj.Labels = map[string]string{"opt": "me"}
	_, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Update(context.Background(), proj, metav1.UpdateOptions{})
	assert.NoError(t, err)

	//Create an app belongs to proj project
	_, err = fixture.RunCli("app", "create", fixture.Name(), "--repo", fixture.RepoURL(fixture.RepoURLTypeFile),
		"--path", guestbookPath, "--project", proj.Name, "--dest-server", common.KubernetesInternalAPIServerAddr, "--dest-namespace", fixture.DeploymentNamespace())
	assert.NoError(t, err)

	//App trying to sync a resource which is not blacked listed anywhere
	_, err = fixture.RunCli("app", "sync", fixture.Name(), "--resource", "apps:Deployment:guestbook-ui", "--timeout", fmt.Sprintf("%v", 10))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Blocked by sync window")

	//app trying to sync a resource which is black listed by global project
	_, err = fixture.RunCli("app", "sync", fixture.Name(), "--resource", ":Service:guestbook-ui", "--timeout", fmt.Sprintf("%v", 10))
	assert.Contains(t, err.Error(), "Blocked by sync window")

}
