package diff

import (
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/stretchr/testify/assert"
	"github.com/yudai/gojsondiff/formatter"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDiff(t *testing.T) {
	leftDep := test.DemoDeployment()
	leftUn := kube.MustToUnstructured(leftDep)

	diffRes := Diff(leftUn, leftUn)
	assert.False(t, diffRes.Diff.Modified())
}

func TestDiffWithNils(t *testing.T) {
	dep := test.DemoDeployment()
	resource := kube.MustToUnstructured(dep)

	diffRes := Diff(nil, resource)
	// NOTE: if live is non-nil, and config is nil, this is not considered difference
	// This "difference" is checked at the comparator.
	assert.False(t, diffRes.Diff.Modified())

	diffRes = Diff(resource, nil)
	assert.True(t, diffRes.Diff.Modified())
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

// TestThreeWayDiff will perform a diff when there is a kubectl.kubernetes.io/last-applied-configuration
// present in the live object.
func TestThreeWayDiff(t *testing.T) {
	// 1. get config and live to be the same. Both have a foo annotation.
	configDep := test.DemoDeployment()
	configDep.Annotations = map[string]string{
		"foo": "bar",
	}
	liveDep := configDep.DeepCopy()

	// 2. add a extra field to the live. this simulates kubernetes adding default values in the
	// object. We should not consider defaulted values as a difference
	liveDep.Annotations["some-default-val"] = "default"
	configUn := kube.MustToUnstructured(configDep)
	liveUn := kube.MustToUnstructured(liveDep)
	res := Diff(configUn, liveUn)
	assert.False(t, res.Modified)

	// 3. Add a last-applied-configuration annotation in the live. There should still not be any
	// difference
	configBytes, err := json.Marshal(configDep)
	assert.Nil(t, err)
	liveDep.Annotations[v1.LastAppliedConfigAnnotation] = string(configBytes)
	configUn = kube.MustToUnstructured(configDep)
	liveUn = kube.MustToUnstructured(liveDep)
	res = Diff(configUn, liveUn)
	assert.False(t, res.Modified)

	// 4. Remove the foo annotation from config and perform the diff again. We should detect a
	// difference since three-way diff detects the removal of a managed field
	delete(configDep.Annotations, "foo")
	configUn = kube.MustToUnstructured(configDep)
	liveUn = kube.MustToUnstructured(liveDep)
	res = Diff(configUn, liveUn)
	assert.True(t, res.Modified)

	// 5. Just to prove three way diff incorporates last-applied-configuration, remove the
	// last-applied-configuration annotation from the live object, and redo the diff. This time,
	// the diff will report not modified (because we have no way of knowing what was a defaulted
	// field without this annotation)
	delete(liveDep.Annotations, v1.LastAppliedConfigAnnotation)
	configUn = kube.MustToUnstructured(configDep)
	liveUn = kube.MustToUnstructured(liveDep)
	res = Diff(configUn, liveUn)
	formatOpts := formatter.AsciiFormatterConfig{
		Coloring: terminal.IsTerminal(int(os.Stdout.Fd())),
	}
	ascii, err := res.ASCIIFormat(configUn, formatOpts)
	assert.Nil(t, err)
	if ascii != "" {
		log.Println(ascii)
	}
	assert.False(t, res.Modified)
}
