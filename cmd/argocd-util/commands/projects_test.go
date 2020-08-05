package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
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
	clientset := fake.NewSimpleClientset(newProj("foo", "test"), newProj("bar", "test"))

	modification, err := getModification("set", "*", "*", "allow")
	assert.NoError(t, err)
	err = updateProjects(clientset.ArgoprojV1alpha1().AppProjects(namespace), "ba*", "*", "set", modification, false)
	assert.NoError(t, err)

	fooProj, err := clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(context.Background(), "foo", v1.GetOptions{})
	assert.NoError(t, err)
	assert.Len(t, fooProj.Spec.Roles[0].Policies, 0)

	barProj, err := clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(context.Background(), "bar", v1.GetOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, barProj.Spec.Roles[0].Policies, []string{"p, proj:bar:test, *, set, bar/*, allow"})
}

func TestUpdateProjects_FindMatchingRole(t *testing.T) {
	clientset := fake.NewSimpleClientset(newProj("proj", "foo", "bar"))

	modification, err := getModification("set", "*", "*", "allow")
	assert.NoError(t, err)
	err = updateProjects(clientset.ArgoprojV1alpha1().AppProjects(namespace), "*", "fo*", "set", modification, false)
	assert.NoError(t, err)

	proj, err := clientset.ArgoprojV1alpha1().AppProjects(namespace).Get(context.Background(), "proj", v1.GetOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, proj.Spec.Roles[0].Policies, []string{"p, proj:proj:foo, *, set, proj/*, allow"})
	assert.Len(t, proj.Spec.Roles[1].Policies, 0)
}

func TestGetModification_SetPolicy(t *testing.T) {
	modification, err := getModification("set", "*", "*", "allow")
	assert.NoError(t, err)
	policy := modification("proj", "myaction")
	assert.Equal(t, "*, myaction, proj/*, allow", policy)
}

func TestGetModification_RemovePolicy(t *testing.T) {
	modification, err := getModification("remove", "*", "*", "allow")
	assert.NoError(t, err)
	policy := modification("proj", "myaction")
	assert.Equal(t, "", policy)
}

func TestGetModification_NotSupported(t *testing.T) {
	_, err := getModification("bar", "*", "*", "allow")
	assert.Errorf(t, err, "modification bar is not supported")
}
