package diff

import (
	"testing"

	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDiff(t *testing.T) {
	leftDep := test.DemoDeployment()
	leftUn := kube.MustToUnstructured(leftDep)

	diffRes := Diff(leftUn, leftUn)
	assert.False(t, diffRes.Diff.Modified())
}

func TestDiffArraySame(t *testing.T) {
	leftDep := test.DemoDeployment()
	rightDep := leftDep.DeepCopy()

	leftUn := kube.MustToUnstructured(leftDep)
	rightUn := kube.MustToUnstructured(rightDep)

	left := []*unstructured.Unstructured{leftUn}
	right := []*unstructured.Unstructured{rightUn}
	diffResList, err := DiffArray(left, right)
	assert.Nil(t, err)
	assert.False(t, diffResList.Modified)
}

func TestDiffArrayAdditions(t *testing.T) {
	leftDep := test.DemoDeployment()
	rightDep := leftDep.DeepCopy()
	rightDep.Status.Replicas = 1

	leftUn := kube.MustToUnstructured(leftDep)
	rightUn := kube.MustToUnstructured(rightDep)

	left := []*unstructured.Unstructured{leftUn}
	right := []*unstructured.Unstructured{rightUn}
	diffResList, err := DiffArray(left, right)
	assert.Nil(t, err)
	assert.False(t, diffResList.Modified)
}

func TestDiffArrayModification(t *testing.T) {
	leftDep := test.DemoDeployment()
	rightDep := leftDep.DeepCopy()
	ten := int32(10)
	rightDep.Spec.Replicas = &ten

	leftUn := kube.MustToUnstructured(leftDep)
	rightUn := kube.MustToUnstructured(rightDep)

	left := []*unstructured.Unstructured{leftUn}
	right := []*unstructured.Unstructured{rightUn}
	diffResList, err := DiffArray(left, right)
	assert.Nil(t, err)
	assert.True(t, diffResList.Modified)
}
