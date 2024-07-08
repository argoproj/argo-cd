package admin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
)

const (
	namespace = "default"
)

func newProj(name string, roleNames ...string) *v1alpha1.AppProject {
	var roles []v1alpha1.ProjectRole
	for i := range roleNames {
		roles = append(roles, v1alpha1.ProjectRole{Name: roleNames[i]})
	}
	return &v1alpha1.AppProject{ObjectMeta: v1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}, Spec: v1alpha1.AppProjectSpec{
		Roles: roles,
	}}
}

func TestUpdateProjects_FindMatchingProject(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset(newProj("foo", "test"), newProj("bar", "test"))

	modification, err := getModification("set", "*", "*", "allow")
	require.NoError(t, err)
	err = updateProjects(ctx, clientset.ArgoprojV1alpha1().AppProjects(namespace), "ba*", "*", "set", modification, false)
	require.NoError(t, err)

	fooProj, err := clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, "foo", v1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, fooProj.Spec.Roles[0].Policies)

	barProj, err := clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, "bar", v1.GetOptions{})
	require.NoError(t, err)
	assert.EqualValues(t, []string{"p, proj:bar:test, *, set, bar/*, allow"}, barProj.Spec.Roles[0].Policies)
}

func TestUpdateProjects_FindMatchingRole(t *testing.T) {
	ctx := context.Background()

	clientset := fake.NewSimpleClientset(newProj("proj", "foo", "bar"))

	modification, err := getModification("set", "*", "*", "allow")
	require.NoError(t, err)
	err = updateProjects(ctx, clientset.ArgoprojV1alpha1().AppProjects(namespace), "*", "fo*", "set", modification, false)
	require.NoError(t, err)

	proj, err := clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(ctx, "proj", v1.GetOptions{})
	require.NoError(t, err)
	assert.EqualValues(t, []string{"p, proj:proj:foo, *, set, proj/*, allow"}, proj.Spec.Roles[0].Policies)
	assert.Empty(t, proj.Spec.Roles[1].Policies)
}

func TestGetModification_SetPolicy(t *testing.T) {
	modification, err := getModification("set", "*", "*", "allow")
	require.NoError(t, err)
	policy := modification("proj", "myaction")
	assert.Equal(t, "*, myaction, proj/*, allow", policy)
}

func TestGetModification_RemovePolicy(t *testing.T) {
	modification, err := getModification("remove", "*", "*", "allow")
	require.NoError(t, err)
	policy := modification("proj", "myaction")
	assert.Equal(t, "", policy)
}

func TestGetModification_NotSupported(t *testing.T) {
	_, err := getModification("bar", "*", "*", "allow")
	assert.Errorf(t, err, "modification bar is not supported")
}
