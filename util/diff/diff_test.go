package diff

import (
	"testing"

	"github.com/argoproj/argo-cd/test"
	"github.com/stretchr/testify/assert"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDiff(t *testing.T) {
	leftDep := test.DemoDeployment()
	leftMap, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(leftDep)
	assert.Nil(t, err)
	leftUn := unstructured.Unstructured{Object: leftMap}
	left := &leftUn

	diffRes := Diff(left, left)
	assert.False(t, diffRes.Diff.Modified())
	assert.Nil(t, diffRes.AdditionsOnly)
}

func TestDiffArraySame(t *testing.T) {
	leftDep := test.DemoDeployment()
	rightDep := leftDep.DeepCopy()

	leftMap, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(leftDep)
	assert.Nil(t, err)
	leftUn := unstructured.Unstructured{Object: leftMap}
	rightMap, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(rightDep)
	assert.Nil(t, err)
	rightUn := unstructured.Unstructured{Object: rightMap}

	left := []*unstructured.Unstructured{&leftUn}
	right := []*unstructured.Unstructured{&rightUn}
	diffResList, err := DiffArray(left, right)
	assert.Nil(t, err)
	assert.False(t, diffResList.Modified)
	assert.Nil(t, diffResList.AdditionsOnly)
}

func TestDiffArrayAdditions(t *testing.T) {
	leftDep := test.DemoDeployment()
	rightDep := leftDep.DeepCopy()
	rightDep.Status.Replicas = 1

	leftMap, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(leftDep)
	assert.Nil(t, err)
	leftUn := unstructured.Unstructured{Object: leftMap}
	rightMap, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(rightDep)
	assert.Nil(t, err)
	rightUn := unstructured.Unstructured{Object: rightMap}

	left := []*unstructured.Unstructured{&leftUn}
	right := []*unstructured.Unstructured{&rightUn}
	diffResList, err := DiffArray(left, right)
	assert.Nil(t, err)
	assert.True(t, diffResList.Modified)
	assert.True(t, *diffResList.AdditionsOnly)
}

func TestDiffArrayModification(t *testing.T) {
	leftDep := test.DemoDeployment()
	rightDep := leftDep.DeepCopy()
	ten := int32(10)
	rightDep.Spec.Replicas = &ten

	leftMap, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(leftDep)
	assert.Nil(t, err)
	leftUn := unstructured.Unstructured{Object: leftMap}
	rightMap, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(rightDep)
	assert.Nil(t, err)
	rightUn := unstructured.Unstructured{Object: rightMap}

	left := []*unstructured.Unstructured{&leftUn}
	right := []*unstructured.Unstructured{&rightUn}
	diffResList, err := DiffArray(left, right)
	assert.Nil(t, err)
	assert.True(t, diffResList.Modified)
	assert.False(t, *diffResList.AdditionsOnly)
}
