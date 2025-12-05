package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
)

const (
	namespace = "default"
)

func newProj(name string, roleNames ...string) *v1alpha1.AppProject {
	var roles []v1alpha1.ProjectRole
	for i := range roleNames {
		roles = append(roles, v1alpha1.ProjectRole{Name: roleNames[i]})
	}
	return &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}, Spec: v1alpha1.AppProjectSpec{
		Roles: roles,
	}}
}

func TestUpdateProjects_FindMatchingProject(t *testing.T) {
	ctx := t.Context()

	clientset := fake.NewSimpleClientset(newProj("foo", "test"), newProj("bar", "test"))

	modification, err := getModification("set", "applications", "*", "allow")
	require.NoError(t, err)
	err = updateProjects(ctx, clientset.ArgoprojV1alpha1().AppProjects(namespace), "ba*", "*", "set", "*", modification, false)
	require.NoError(t, err)

	fooProj, err := clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, "foo", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, fooProj.Spec.Roles[0].Policies)

	barProj, err := clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, "bar", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"p, proj:bar:test, applications, set, bar/*, allow"}, barProj.Spec.Roles[0].Policies)
}

func TestUpdateProjects_FindMatchingRole(t *testing.T) {
	ctx := t.Context()

	clientset := fake.NewSimpleClientset(newProj("proj", "foo", "bar"))

	modification, err := getModification("set", "applications", "*", "allow")
	require.NoError(t, err)
	err = updateProjects(ctx, clientset.ArgoprojV1alpha1().AppProjects(namespace), "*", "fo*", "set", "*", modification, false)
	require.NoError(t, err)

	proj, err := clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, "proj", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"p, proj:proj:foo, applications, set, proj/*, allow"}, proj.Spec.Roles[0].Policies)
	assert.Empty(t, proj.Spec.Roles[1].Policies)
}

func TestUpdateProjects_MultiplePolicies(t *testing.T) {
	ctx := t.Context()

	proj := newProj("proj", "foo")
	proj.Spec.Roles[0].Policies = []string{
		"p, proj:proj:foo, applications, get, proj/*, allow",
		"p, proj:proj:foo, applicationsets, get, proj/*, allow",
	}
	clientset := fake.NewSimpleClientset(proj)

	// add policy
	modification, err := getModification("set", "logs", "*", "allow")
	require.NoError(t, err)
	err = updateProjects(ctx, clientset.ArgoprojV1alpha1().AppProjects(namespace), "*", "fo*", "get", "logs", modification, false)
	require.NoError(t, err)

	proj, err = clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, "proj", metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, "p, proj:proj:foo, logs, get, proj/*, allow", proj.Spec.Roles[0].Policies[2])
	// remove policy
	modification, err = getModification("remove", "logs", "*", "allow")
	require.NoError(t, err)
	err = updateProjects(ctx, clientset.ArgoprojV1alpha1().AppProjects(namespace), "*", "fo*", "get", "logs", modification, false)
	require.NoError(t, err)

	proj, err = clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, "proj", metav1.GetOptions{})
	require.NoError(t, err)

	assert.Len(t, proj.Spec.Roles[0].Policies, 2)
	// update policy

	modification, err = getModification("set", "applications", "*", "deny")
	require.NoError(t, err)
	err = updateProjects(ctx, clientset.ArgoprojV1alpha1().AppProjects(namespace), "*", "fo*", "get", "applications", modification, false)
	require.NoError(t, err)

	proj, err = clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, "proj", metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, "p, proj:proj:foo, applications, get, proj/*, deny", proj.Spec.Roles[0].Policies[0])
}

func TestGetModification_SetPolicy(t *testing.T) {
	modification, err := getModification("set", "logs", "*", "allow")
	require.NoError(t, err)
	policy := modification("proj", "myaction")
	assert.Equal(t, "logs, myaction, proj/*, allow", policy)
}

func TestGetModification_RemovePolicy(t *testing.T) {
	modification, err := getModification("remove", "logs", "*", "allow")
	require.NoError(t, err)
	policy := modification("proj", "myaction")
	assert.Empty(t, policy)
}

func TestGetModification_NotSupported(t *testing.T) {
	_, err := getModification("bar", "logs", "*", "allow")
	assert.Errorf(t, err, "modification bar is not supported")
}

func TestGetModification_ResourceNotSupported(t *testing.T) {
	_, err := getModification("set", "dummy", "*", "allow")
	assert.Errorf(t, err, "flag --resource should be project scoped resource, e.g. 'applications, logs, exec, etc.'")

	_, err = getModification("set", "*", "*", "allow")
	assert.Errorf(t, err, "flag --resource should be project scoped resource, e.g. 'applications, logs, exec, etc.'")
}
