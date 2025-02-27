package diff

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/klog/v2/textlogger"
	openapiproto "k8s.io/kube-openapi/pkg/util/proto"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/gitops-engine/pkg/diff/mocks"
	"github.com/argoproj/gitops-engine/pkg/diff/testdata"
)

func printDiff(result *DiffResult) (string, error) {
	var live unstructured.Unstructured
	if err := json.Unmarshal(result.NormalizedLive, &live); err != nil {
		return "", err
	}
	var target unstructured.Unstructured
	if err := json.Unmarshal(result.PredictedLive, &target); err != nil {
		return "", err
	}
	out, _ := printDiffInternal("diff", &live, &target)
	return string(out), nil
}

// printDiffInternal prints a diff between two unstructured objects using an external diff utility and returns the output.
func printDiffInternal(name string, live *unstructured.Unstructured, target *unstructured.Unstructured) ([]byte, error) {
	tempDir, err := os.MkdirTemp("", "argocd-diff")
	if err != nil {
		return nil, err
	}
	targetFile := filepath.Join(tempDir, name)
	var targetData []byte
	if target != nil {
		targetData, err = yaml.Marshal(target)
		if err != nil {
			return nil, err
		}
	}
	err = os.WriteFile(targetFile, targetData, 0o644)
	if err != nil {
		return nil, err
	}
	liveFile := filepath.Join(tempDir, name+"-live.yaml")
	liveData := []byte("")
	if live != nil {
		liveData, err = yaml.Marshal(live)
		if err != nil {
			return nil, err
		}
	}
	err = os.WriteFile(liveFile, liveData, 0o644)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("diff", liveFile, targetFile)
	return cmd.Output()
}

func toUnstructured(obj any) (*unstructured.Unstructured, error) {
	uObj, err := runtime.NewTestUnstructuredConverter(equality.Semantic).ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: uObj}, nil
}

func mustToUnstructured(obj any) *unstructured.Unstructured {
	un, err := toUnstructured(obj)
	if err != nil {
		panic(err)
	}
	return un
}

func unmarshalFile(path string) *unstructured.Unstructured {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var un unstructured.Unstructured
	err = json.Unmarshal(data, &un.Object)
	if err != nil {
		panic(err)
	}
	return &un
}

func newDeployment() *appsv1.Deployment {
	var two int32 = 2
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &two,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "demo",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "demo",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "demo",
							Image: "gcr.io/kuar-demo/kuard-amd64:1",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}

func diff(t *testing.T, config, live *unstructured.Unstructured, options ...Option) *DiffResult {
	t.Helper()
	res, err := Diff(config, live, options...)
	assert.NoError(t, err)
	return res
}

func TestDiff(t *testing.T) {
	leftDep := newDeployment()
	leftUn := mustToUnstructured(leftDep)

	diffRes := diff(t, leftUn, leftUn, diffOptionsForTest()...)
	assert.False(t, diffRes.Modified)
	ascii, err := printDiff(diffRes)
	require.NoError(t, err)
	if ascii != "" {
		t.Log(ascii)
	}
}

func TestDiff_KnownTypeInvalidValue(t *testing.T) {
	leftDep := newDeployment()
	leftUn := mustToUnstructured(leftDep)
	require.NoError(t, unstructured.SetNestedField(leftUn.Object, "badValue", "spec", "revisionHistoryLimit"))

	t.Run("NoDifference", func(t *testing.T) {
		diffRes := diff(t, leftUn, leftUn, diffOptionsForTest()...)
		assert.False(t, diffRes.Modified)
		ascii, err := printDiff(diffRes)
		require.NoError(t, err)
		if ascii != "" {
			t.Log(ascii)
		}
	})

	t.Run("HasDifference", func(t *testing.T) {
		rightUn := leftUn.DeepCopy()
		require.NoError(t, unstructured.SetNestedField(rightUn.Object, "3", "spec", "revisionHistoryLimit"))

		diffRes := diff(t, leftUn, rightUn, diffOptionsForTest()...)
		assert.True(t, diffRes.Modified)
	})
}

func TestDiffWithNils(t *testing.T) {
	dep := newDeployment()
	resource := mustToUnstructured(dep)

	diffRes := diff(t, nil, resource, diffOptionsForTest()...)
	// NOTE: if live is non-nil, and config is nil, this is not considered difference
	// This "difference" is checked at the comparator.
	assert.False(t, diffRes.Modified)
	diffRes, err := TwoWayDiff(nil, resource)
	require.NoError(t, err)
	assert.False(t, diffRes.Modified)

	diffRes = diff(t, resource, nil, diffOptionsForTest()...)
	assert.True(t, diffRes.Modified)
	diffRes, err = TwoWayDiff(resource, nil)
	require.NoError(t, err)
	assert.True(t, diffRes.Modified)
}

func TestDiffNilFieldInLive(t *testing.T) {
	leftDep := newDeployment()
	rightDep := leftDep.DeepCopy()

	leftUn := mustToUnstructured(leftDep)
	rightUn := mustToUnstructured(rightDep)
	err := unstructured.SetNestedField(rightUn.Object, nil, "spec")
	require.NoError(t, err)

	diffRes := diff(t, leftUn, rightUn, diffOptionsForTest()...)
	assert.True(t, diffRes.Modified)
}

func TestDiffArraySame(t *testing.T) {
	leftDep := newDeployment()
	rightDep := leftDep.DeepCopy()

	leftUn := mustToUnstructured(leftDep)
	rightUn := mustToUnstructured(rightDep)

	left := []*unstructured.Unstructured{leftUn}
	right := []*unstructured.Unstructured{rightUn}
	diffResList, err := DiffArray(left, right, diffOptionsForTest()...)
	require.NoError(t, err)
	assert.False(t, diffResList.Modified)
}

func TestDiffArrayAdditions(t *testing.T) {
	leftDep := newDeployment()
	rightDep := leftDep.DeepCopy()
	rightDep.Status.Replicas = 1

	leftUn := mustToUnstructured(leftDep)
	rightUn := mustToUnstructured(rightDep)

	left := []*unstructured.Unstructured{leftUn}
	right := []*unstructured.Unstructured{rightUn}
	diffResList, err := DiffArray(left, right, diffOptionsForTest()...)
	require.NoError(t, err)
	assert.False(t, diffResList.Modified)
}

func TestDiffArrayModification(t *testing.T) {
	leftDep := newDeployment()
	rightDep := leftDep.DeepCopy()
	ten := int32(10)
	rightDep.Spec.Replicas = &ten

	leftUn := mustToUnstructured(leftDep)
	rightUn := mustToUnstructured(rightDep)

	left := []*unstructured.Unstructured{leftUn}
	right := []*unstructured.Unstructured{rightUn}
	diffResList, err := DiffArray(left, right, diffOptionsForTest()...)
	require.NoError(t, err)
	assert.True(t, diffResList.Modified)
}

// TestThreeWayDiff will perform a diff when there is a kubectl.kubernetes.io/last-applied-configuration
// present in the live object.
func TestThreeWayDiff(t *testing.T) {
	// 1. get config and live to be the same. Both have a foo annotation.
	configDep := newDeployment()
	configDep.ObjectMeta.Namespace = ""
	configDep.Annotations = map[string]string{
		"foo": "bar",
	}
	liveDep := configDep.DeepCopy()

	// 2. add a extra field to the live. this simulates kubernetes adding default values in the
	// object. We should not consider defaulted values as a difference
	liveDep.SetNamespace("default")
	configUn := mustToUnstructured(configDep)
	liveUn := mustToUnstructured(liveDep)
	res := diff(t, configUn, liveUn, diffOptionsForTest()...)
	if !assert.False(t, res.Modified) {
		ascii, err := printDiff(res)
		require.NoError(t, err)
		t.Log(ascii)
	}

	// 3. Add a last-applied-configuration annotation in the live. There should still not be any
	// difference
	configBytes, err := json.Marshal(configDep)
	require.NoError(t, err)
	liveDep.Annotations[corev1.LastAppliedConfigAnnotation] = string(configBytes)
	configUn = mustToUnstructured(configDep)
	liveUn = mustToUnstructured(liveDep)
	res = diff(t, configUn, liveUn, diffOptionsForTest()...)
	if !assert.False(t, res.Modified) {
		ascii, err := printDiff(res)
		require.NoError(t, err)
		t.Log(ascii)
	}

	// 4. Remove the foo annotation from config and perform the diff again. We should detect a
	// difference since three-way diff detects the removal of a managed field
	delete(configDep.Annotations, "foo")
	configUn = mustToUnstructured(configDep)
	liveUn = mustToUnstructured(liveDep)
	res = diff(t, configUn, liveUn, diffOptionsForTest()...)
	assert.True(t, res.Modified)

	// 5. Just to prove three way diff incorporates last-applied-configuration, remove the
	// last-applied-configuration annotation from the live object, and redo the diff. This time,
	// the diff will report not modified (because we have no way of knowing what was a defaulted
	// field without this annotation)
	delete(liveDep.Annotations, corev1.LastAppliedConfigAnnotation)
	configUn = mustToUnstructured(configDep)
	liveUn = mustToUnstructured(liveDep)
	res = diff(t, configUn, liveUn, diffOptionsForTest()...)
	ascii, err := printDiff(res)
	require.NoError(t, err)
	if ascii != "" {
		t.Log(ascii)
	}
	assert.False(t, res.Modified)
}

var demoConfig = `
{
  "apiVersion": "v1",
  "kind": "ServiceAccount",
  "metadata": {
    "labels": {
      "app.kubernetes.io/instance": "argocd-demo"
    },
    "name": "argocd-application-controller"
  }
}
`

var demoLive = `
{
  "apiVersion": "v1",
  "kind": "ServiceAccount",
  "metadata": {
    "annotations": {
      "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"ServiceAccount\",\"metadata\":{\"annotations\":{},\"labels\":{\"app.kubernetes.io/instance\":\"argocd-demo\"},\"name\":\"argocd-application-controller\",\"namespace\":\"argocd-demo\"}}\n"
    },
    "creationTimestamp": "2018-04-16T22:08:57Z",
    "labels": {
      "app.kubernetes.io/instance": "argocd-demo"
    },
    "name": "argocd-application-controller",
    "namespace": "argocd-demo",
    "resourceVersion": "7584502",
    "selfLink": "/api/v1/namespaces/argocd-demo/serviceaccounts/argocd-application-controller",
    "uid": "c22bb2b4-41c2-11e8-978a-028445d52ec8"
  },
  "secrets": [
    {
      "name": "argocd-application-controller-token-kfxct"
    }
  ]
}
`

// Tests a real world example
func TestThreeWayDiffExample1(t *testing.T) {
	var configUn, liveUn unstructured.Unstructured
	// NOTE: it is intentional to unmarshal to Unstructured.Object instead of just Unstructured
	// since it catches a case when we comparison fails due to subtle differences in types
	// (e.g. float vs. int)
	err := json.Unmarshal([]byte(demoConfig), &configUn.Object)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(demoLive), &liveUn.Object)
	require.NoError(t, err)
	dr := diff(t, &configUn, &liveUn, diffOptionsForTest()...)
	assert.False(t, dr.Modified)
	ascii, err := printDiff(dr)
	require.NoError(t, err)
	if ascii != "" {
		t.Log(ascii)
	}
}

// Test for ignoring aggregated cluster roles
func TestDiffOptionIgnoreAggregateRoles(t *testing.T) {
	// Test case 1: Ignore option is true, the rules in the role should be ignored
	{
		configUn := unmarshalFile("testdata/aggr-clusterrole-config.json")
		liveUn := unmarshalFile("testdata/aggr-clusterrole-live.json")
		dr := diff(t, configUn, liveUn, IgnoreAggregatedRoles(true))
		assert.False(t, dr.Modified)
		ascii, err := printDiff(dr)
		require.NoError(t, err)
		t.Log(ascii)
	}
	// Test case 2: Ignore option is false, the aggregation should produce a diff
	{
		configUn := unmarshalFile("testdata/aggr-clusterrole-config.json")
		liveUn := unmarshalFile("testdata/aggr-clusterrole-live.json")
		dr := diff(t, configUn, liveUn, IgnoreAggregatedRoles(false))
		assert.True(t, dr.Modified)
		ascii, err := printDiff(dr)
		require.NoError(t, err)
		t.Log(ascii)
	}
}

func TestThreeWayDiffExample2(t *testing.T) {
	configUn := unmarshalFile("testdata/elasticsearch-config.json")
	liveUn := unmarshalFile("testdata/elasticsearch-live.json")
	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	assert.False(t, dr.Modified)
	ascii, err := printDiff(dr)
	require.NoError(t, err)
	t.Log(ascii)
}

// Tests a real world example
func TestThreeWayDiffExample3(t *testing.T) {
	configUn := unmarshalFile("testdata/deployment-config.json")
	liveUn := unmarshalFile("testdata/deployment-live.json")

	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	assert.False(t, dr.Modified)
	ascii, err := printDiff(dr)
	require.NoError(t, err)
	if ascii != "" {
		t.Log(ascii)
	}
}

func TestThreeWayDiffExample4(t *testing.T) {
	configUn := unmarshalFile("testdata/mutatingwebhookconfig-config.json")
	liveUn := unmarshalFile("testdata/mutatingwebhookconfig-live.json")

	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	assert.False(t, dr.Modified)
	ascii, err := printDiff(dr)
	require.NoError(t, err)
	if ascii != "" {
		t.Log(ascii)
	}
}

// TestThreeWayDiffExample2WithDifference is same as TestThreeWayDiffExample2 but with differences
func TestThreeWayDiffExample2WithDifference(t *testing.T) {
	configUn := unmarshalFile("testdata/elasticsearch-config.json")
	liveUn := unmarshalFile("testdata/elasticsearch-live.json")
	labels := configUn.GetLabels()
	// add a new label
	labels["foo"] = "bar"
	// modify a label
	labels["chart"] = "elasticsearch-1.7.1"
	// remove an existing label
	delete(labels, "release")
	configUn.SetLabels(labels)

	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	assert.True(t, dr.Modified)
	ascii, err := printDiff(dr)
	require.NoError(t, err, ascii)
	t.Log(ascii)

	// Check that we indicate missing/extra/changed correctly
	showsMissing := 0
	showsExtra := 0
	showsChanged := 0
	for _, line := range strings.Split(ascii, "\n") {
		if strings.HasPrefix(line, `>     foo: bar`) {
			showsMissing++
		}
		if strings.HasPrefix(line, `<     release: elasticsearch4`) {
			showsExtra++
		}
		if strings.HasPrefix(line, `>     chart: elasticsearch-1.7.1`) {
			showsChanged++
		}
		if strings.HasPrefix(line, `<     chart: elasticsearch-1.7.0`) {
			showsChanged++
		}
	}
	assert.Equal(t, 1, showsMissing)
	assert.Equal(t, 1, showsExtra)
	assert.Equal(t, 2, showsChanged)
}

func TestThreeWayDiffExplicitNamespace(t *testing.T) {
	configUn := unmarshalFile("testdata/spinnaker-sa-config.json")
	liveUn := unmarshalFile("testdata/spinnaker-sa-live.json")
	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	assert.False(t, dr.Modified)
	ascii, err := printDiff(dr)
	require.NoError(t, err)
	t.Log(ascii)
}

func TestDiffResourceWithInvalidField(t *testing.T) {
	// Diff(...) should not silently discard invalid fields (fields that are not present in the underlying k8s resource).

	leftDep := `{
			"apiVersion": "v1",
			"kind": "ConfigMap",
			"metadata": {
			  "name": "invalid-cm"
			},
			"invalidKey": "asdf"
		  }`
	var leftUn unstructured.Unstructured
	err := json.Unmarshal([]byte(leftDep), &leftUn.Object)
	if err != nil {
		panic(err)
	}

	rightUn := leftUn.DeepCopy()
	unstructured.RemoveNestedField(rightUn.Object, "invalidKey")

	diffRes := diff(t, &leftUn, rightUn, diffOptionsForTest()...)
	assert.True(t, diffRes.Modified)
	ascii, err := printDiff(diffRes)
	require.NoError(t, err)

	assert.Contains(t, ascii, "invalidKey")
	if ascii != "" {
		t.Log(ascii)
	}
}

func TestRemoveNamespaceAnnotation(t *testing.T) {
	obj := removeNamespaceAnnotation(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":      "test",
			"namespace": "default",
		},
	}})
	assert.Equal(t, "", obj.GetNamespace())

	obj = removeNamespaceAnnotation(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":        "test",
			"namespace":   "default",
			"annotations": make(map[string]any),
		},
	}})
	assert.Equal(t, "", obj.GetNamespace())
	assert.Nil(t, obj.GetAnnotations())

	obj = removeNamespaceAnnotation(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":        "test",
			"namespace":   "default",
			"annotations": "wrong value",
		},
	}})
	assert.Equal(t, "", obj.GetNamespace())
	val, _, _ := unstructured.NestedString(obj.Object, "metadata", "annotations")
	assert.Equal(t, "wrong value", val)
}

const customObjConfig = `
apiVersion: foo.io/v1
kind: Foo
metadata:
  name: my-foo
  namespace: kube-system
spec:
  foo: bar
`

const customObjLive = `
apiVersion: foo.io/v1
kind: Foo
metadata:
  creationTimestamp: 2018-07-17 09:17:05 UTC
  name: my-foo
  resourceVersion: '10308211'
  selfLink: "/apis/rbac.authorization.k8s.io/v1/clusterroles/argocd-manager-role"
  uid: 2c3d5405-89a2-11e8-aff0-42010a8a0fc6
spec:
  foo: bar
`

func TestIgnoreNamespaceForClusterScopedResources(t *testing.T) {
	var configUn unstructured.Unstructured
	var liveUn unstructured.Unstructured
	err := yaml.Unmarshal([]byte(customObjLive), &liveUn)
	require.NoError(t, err)
	err = yaml.Unmarshal([]byte(customObjConfig), &configUn)
	require.NoError(t, err)
	dr := diff(t, &configUn, &liveUn, diffOptionsForTest()...)
	assert.False(t, dr.Modified)
}

const secretConfig = `
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
type: Opaque
stringData:
  foo: bar
  bar: "1234"
data:
  baz: cXV4
`

const secretLive = `
apiVersion: v1
kind: Secret
metadata:
  creationTimestamp: 2018-11-19T11:30:40Z
  name: my-secret
  namespace: argocd
  resourceVersion: "25848035"
  selfLink: /api/v1/namespaces/argocd/secrets/my-secret
  uid: 8b4a2766-ebee-11e8-93c0-42010a8a0013
type: Opaque
data:
  foo: YmFy
  bar: MTIzNA==
  baz: cXV4
`

func TestSecretStringData(t *testing.T) {
	var err error
	var configUn unstructured.Unstructured
	err = yaml.Unmarshal([]byte(secretConfig), &configUn)
	require.NoError(t, err)

	var liveUn unstructured.Unstructured
	err = yaml.Unmarshal([]byte(secretLive), &liveUn)
	require.NoError(t, err)

	dr := diff(t, &configUn, &liveUn, diffOptionsForTest()...)
	if !assert.False(t, dr.Modified) {
		ascii, err := printDiff(dr)
		require.NoError(t, err)
		t.Log(ascii)
	}
}

// This is invalid because foo is a number, not a string
const secretInvalidConfig = `
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
type: Opaque
stringData:
  foo: 1234
`

const secretInvalidLive = `
apiVersion: v1
kind: Secret
metadata:
  creationTimestamp: 2018-11-19T11:30:40Z
  name: my-secret
  namespace: argocd
  resourceVersion: "25848035"
  selfLink: /api/v1/namespaces/argocd/secrets/my-secret
  uid: 8b4a2766-ebee-11e8-93c0-42010a8a0013
type: Opaque
data:
  foo: MTIzNA==
`

func TestInvalidSecretStringData(t *testing.T) {
	var err error
	var configUn unstructured.Unstructured
	err = yaml.Unmarshal([]byte(secretInvalidConfig), &configUn)
	require.NoError(t, err)

	var liveUn unstructured.Unstructured
	err = yaml.Unmarshal([]byte(secretInvalidLive), &liveUn)
	require.NoError(t, err)

	dr := diff(t, &configUn, nil, diffOptionsForTest()...)
	assert.True(t, dr.Modified)
}

func TestNullSecretData(t *testing.T) {
	configUn := unmarshalFile("testdata/wordpress-config.json")
	liveUn := unmarshalFile("testdata/wordpress-live.json")
	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	if !assert.False(t, dr.Modified) {
		ascii, err := printDiff(dr)
		require.NoError(t, err)
		t.Log(ascii)
	}
}

// TestRedactedSecretData tests we are able to perform diff on redacted secret data, which has
// invalid characters (*) for the the data byte array field.
func TestRedactedSecretData(t *testing.T) {
	configUn := unmarshalFile("testdata/wordpress-config.json")
	liveUn := unmarshalFile("testdata/wordpress-live.json")
	configData := configUn.Object["data"].(map[string]any)
	liveData := liveUn.Object["data"].(map[string]any)
	configData["wordpress-password"] = "++++++++"
	configData["smtp-password"] = "++++++++"
	liveData["wordpress-password"] = "++++++++++++"
	liveData["smtp-password"] = "++++++++++++"
	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	if !assert.True(t, dr.Modified) {
		ascii, err := printDiff(dr)
		require.NoError(t, err)
		t.Log(ascii)
	}
}

func TestNullRoleRule(t *testing.T) {
	configUn := unmarshalFile("testdata/grafana-clusterrole-config.json")
	liveUn := unmarshalFile("testdata/grafana-clusterrole-live.json")
	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	if !assert.False(t, dr.Modified) {
		ascii, err := printDiff(dr)
		require.NoError(t, err)
		t.Log(ascii)
	}
}

func TestNullCreationTimestamp(t *testing.T) {
	configUn := unmarshalFile("testdata/sealedsecret-config.json")
	liveUn := unmarshalFile("testdata/sealedsecret-live.json")
	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	if !assert.False(t, dr.Modified) {
		ascii, err := printDiff(dr)
		require.NoError(t, err)
		t.Log(ascii)
	}
}

func TestUnsortedEndpoints(t *testing.T) {
	configUn := unmarshalFile("testdata/endpoints-config.json")
	liveUn := unmarshalFile("testdata/endpoints-live.json")
	dr := diff(t, configUn, liveUn, diffOptionsForTest()...)
	if !assert.False(t, dr.Modified) {
		ascii, err := printDiff(dr)
		require.NoError(t, err)
		t.Log(ascii)
	}
}

func buildGVKParser(t *testing.T) *managedfields.GvkParser {
	t.Helper()
	document := &openapi_v2.Document{}
	require.NoErrorf(t, proto.Unmarshal(testdata.OpenAPIV2Doc, document), "error unmarshaling openapi doc")
	models, err := openapiproto.NewOpenAPIData(document)
	require.NoErrorf(t, err, "error building openapi data: %s", err)

	gvkParser, err := managedfields.NewGVKParser(models, false)
	require.NoErrorf(t, err, "error building gvkParser: %s", err)
	return gvkParser
}

func TestStructuredMergeDiff(t *testing.T) {
	buildParams := func(live, config *unstructured.Unstructured) *SMDParams {
		gvkParser := buildGVKParser(t)
		manager := "argocd-controller"
		return &SMDParams{
			config:    config,
			live:      live,
			gvkParser: gvkParser,
			manager:   manager,
		}
	}

	t.Run("will apply default values", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.ServiceLiveYAML)
		desiredState := StrToUnstructured(testdata.ServiceConfigYAML)
		params := buildParams(liveState, desiredState)

		// when
		result, err := structuredMergeDiff(params)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified)
		predictedSVC := YamlToSvc(t, result.PredictedLive)
		liveSVC := YamlToSvc(t, result.NormalizedLive)
		require.NotNil(t, predictedSVC.Spec.InternalTrafficPolicy)
		require.NotNil(t, liveSVC.Spec.InternalTrafficPolicy)
		assert.Equal(t, "Cluster", string(*predictedSVC.Spec.InternalTrafficPolicy))
		assert.Equal(t, "Cluster", string(*liveSVC.Spec.InternalTrafficPolicy))
		assert.Empty(t, predictedSVC.Annotations[AnnotationLastAppliedConfig])
		assert.Empty(t, liveSVC.Annotations[AnnotationLastAppliedConfig])
	})
	t.Run("will remove entries in list", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.ServiceLiveYAML)
		desiredState := StrToUnstructured(testdata.ServiceConfigWith2Ports)
		params := buildParams(liveState, desiredState)

		// when
		result, err := structuredMergeDiff(params)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified)
		svc := YamlToSvc(t, result.PredictedLive)
		assert.Len(t, svc.Spec.Ports, 2)
	})
	t.Run("will remove previously added fields not present in desired state", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.LiveServiceWithTypeYAML)
		desiredState := StrToUnstructured(testdata.ServiceConfigYAML)
		params := buildParams(liveState, desiredState)

		// when
		result, err := structuredMergeDiff(params)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified)
		svc := YamlToSvc(t, result.PredictedLive)
		assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	})
	t.Run("will apply service with multiple ports", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.ServiceLiveYAML)
		desiredState := StrToUnstructured(testdata.ServiceConfigWithSamePortsYAML)
		params := buildParams(liveState, desiredState)

		// when
		result, err := structuredMergeDiff(params)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified)
		svc := YamlToSvc(t, result.PredictedLive)
		assert.Len(t, svc.Spec.Ports, 5)
	})
	t.Run("will apply deployment defaults correctly", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.DeploymentLiveYAML)
		desiredState := StrToUnstructured(testdata.DeploymentConfigYAML)
		params := buildParams(liveState, desiredState)

		// when
		result, err := structuredMergeDiff(params)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Modified)
		deploy := YamlToDeploy(t, result.PredictedLive)
		assert.Len(t, deploy.Spec.Template.Spec.Containers, 1)
		assert.Equal(t, "0", deploy.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String())
		assert.Equal(t, "0", deploy.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String())
		assert.Equal(t, "0", deploy.Spec.Template.Spec.Containers[0].Resources.Requests.Storage().String())
		assert.Equal(t, "0", deploy.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String())
		assert.Equal(t, "0", deploy.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String())
		assert.Equal(t, "0", deploy.Spec.Template.Spec.Containers[0].Resources.Limits.Storage().String())
		require.NotNil(t, deploy.Spec.Strategy.RollingUpdate)
		expectedMaxSurge := &intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "25%",
		}
		assert.Equal(t, expectedMaxSurge, deploy.Spec.Strategy.RollingUpdate.MaxSurge)
		assert.Equal(t, "ClusterFirst", string(deploy.Spec.Template.Spec.DNSPolicy))
	})
}

func TestServerSideDiff(t *testing.T) {
	buildOpts := func(predictedLive string) []Option {
		gvkParser := buildGVKParser(t)
		manager := "argocd-controller"
		dryRunner := mocks.NewServerSideDryRunner(t)

		dryRunner.On("Run", mock.Anything, mock.AnythingOfType("*unstructured.Unstructured"), manager).
			Return(func(_ context.Context, _ *unstructured.Unstructured, _ string) (string, error) {
				return predictedLive, nil
			})
		opts := []Option{
			WithGVKParser(gvkParser),
			WithManager(manager),
			WithServerSideDryRunner(dryRunner),
		}

		return opts
	}

	t.Run("will ignore modifications done by mutation webhook by default", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.ServiceLiveYAMLSSD)
		desiredState := StrToUnstructured(testdata.ServiceConfigYAMLSSD)
		opts := buildOpts(testdata.ServicePredictedLiveJSONSSD)

		// when
		result, err := serverSideDiff(desiredState, liveState, opts...)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified)
		predictedSVC := YamlToSvc(t, result.PredictedLive)
		liveSVC := YamlToSvc(t, result.NormalizedLive)
		require.NotNil(t, predictedSVC.Spec.InternalTrafficPolicy)
		require.NotNil(t, liveSVC.Spec.InternalTrafficPolicy)
		assert.Equal(t, "Cluster", string(*predictedSVC.Spec.InternalTrafficPolicy))
		assert.Equal(t, "Cluster", string(*liveSVC.Spec.InternalTrafficPolicy))
		assert.Empty(t, predictedSVC.Annotations[AnnotationLastAppliedConfig])
		assert.Empty(t, liveSVC.Annotations[AnnotationLastAppliedConfig])
		assert.Empty(t, predictedSVC.Labels["event"])
	})

	t.Run("will test removing some field with undoing changes done by webhook", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.Deployment2LiveYAML)
		desiredState := StrToUnstructured(testdata.Deployment2ConfigYAML)
		opts := buildOpts(testdata.Deployment2PredictedLiveJSONSSD)

		// when
		result, err := serverSideDiff(desiredState, liveState, opts...)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified)
		predictedDeploy := YamlToDeploy(t, result.PredictedLive)
		liveDeploy := YamlToDeploy(t, result.NormalizedLive)
		assert.Len(t, predictedDeploy.Spec.Template.Spec.Containers, 1)
		assert.Len(t, liveDeploy.Spec.Template.Spec.Containers, 1)
		assert.Equal(t, "500m", predictedDeploy.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String())
		assert.Equal(t, "512Mi", predictedDeploy.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String())
		assert.Equal(t, "500m", liveDeploy.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String())
		assert.Equal(t, "512Mi", liveDeploy.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String())
	})

	t.Run("will include mutation webhook modifications", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.ServiceLiveYAMLSSD)
		desiredState := StrToUnstructured(testdata.ServiceConfigYAMLSSD)
		opts := buildOpts(testdata.ServicePredictedLiveJSONSSD)
		opts = append(opts, WithIgnoreMutationWebhook(false))

		// when
		result, err := serverSideDiff(desiredState, liveState, opts...)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified)
		predictedSVC := YamlToSvc(t, result.PredictedLive)
		liveSVC := YamlToSvc(t, result.NormalizedLive)
		require.NotNil(t, predictedSVC.Spec.InternalTrafficPolicy)
		require.NotNil(t, liveSVC.Spec.InternalTrafficPolicy)
		assert.Equal(t, "Cluster", string(*predictedSVC.Spec.InternalTrafficPolicy))
		assert.Equal(t, "Cluster", string(*liveSVC.Spec.InternalTrafficPolicy))
		assert.Empty(t, predictedSVC.Annotations[AnnotationLastAppliedConfig])
		assert.Empty(t, liveSVC.Annotations[AnnotationLastAppliedConfig])
		assert.NotEmpty(t, predictedSVC.Labels["event"])
	})

	t.Run("will include nested fields like ports and env", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.DeploymentNestedLiveYAMLSSD)
		desiredState := StrToUnstructured(testdata.DeploymentNestedConfigYAMLSSD)
		opts := buildOpts(testdata.DeploymentNestedPredictedLiveJSONSSD)

		// when
		result, err := serverSideDiff(desiredState, liveState, opts...)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified)

		predictedDeploy := YamlToDeploy(t, result.PredictedLive)
		liveDeploy := YamlToDeploy(t, result.NormalizedLive)

		// Check ports
		assert.Len(t, predictedDeploy.Spec.Template.Spec.Containers[0].Ports, 2)
		assert.Len(t, liveDeploy.Spec.Template.Spec.Containers[0].Ports, 1)
		assert.Equal(t, int32(80), predictedDeploy.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		assert.Equal(t, int32(443), predictedDeploy.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort)

		// Check env
		assert.Len(t, predictedDeploy.Spec.Template.Spec.Containers[0].Env, 2)
		assert.Len(t, liveDeploy.Spec.Template.Spec.Containers[0].Env, 1)
		assert.Equal(t, "ENV_VAR1", predictedDeploy.Spec.Template.Spec.Containers[0].Env[0].Name)
		assert.Equal(t, "ENV_VAR2", predictedDeploy.Spec.Template.Spec.Containers[0].Env[1].Name)
	})

	t.Run("will add an extra container using kubectl apply and include mutation webhook", func(t *testing.T) {
		// given
		t.Parallel()
		liveState := StrToUnstructured(testdata.DeploymentApplyLiveYAMLSSD)
		desiredState := StrToUnstructured(testdata.DeploymentApplyConfigYAMLSSD)
		opts := buildOpts(testdata.DeploymentApplyPredictedLiveJSONSSD)

		// when
		result, err := serverSideDiff(desiredState, liveState, opts...)

		// then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Modified)

		predictedDeploy := YamlToDeploy(t, result.PredictedLive)
		liveDeploy := YamlToDeploy(t, result.NormalizedLive)

		// Check ports are shown in diff and ensure mutation webhook is not shown
		assert.Len(t, predictedDeploy.Spec.Template.Spec.Containers[0].Ports, 2)
		assert.Len(t, liveDeploy.Spec.Template.Spec.Containers[0].Ports, 1)
		assert.Equal(t, int32(80), predictedDeploy.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		assert.Equal(t, int32(40), predictedDeploy.Spec.Template.Spec.Containers[0].Ports[1].ContainerPort)
		assert.Empty(t, predictedDeploy.Annotations[AnnotationLastAppliedConfig])
		assert.Empty(t, liveDeploy.Annotations[AnnotationLastAppliedConfig])
	})
}

func createSecret(data map[string]string) *unstructured.Unstructured {
	secret := corev1.Secret{TypeMeta: metav1.TypeMeta{Kind: "Secret"}}
	if data != nil {
		secret.Data = make(map[string][]byte)
		for k, v := range data {
			secret.Data[k] = []byte(v)
		}
	}

	return mustToUnstructured(&secret)
}

func secretData(obj *unstructured.Unstructured) map[string]any {
	data, _, _ := unstructured.NestedMap(obj.Object, "data")
	return data
}

var (
	replacement1 = strings.Repeat("+", 8)
	replacement2 = strings.Repeat("+", 12)
	replacement3 = strings.Repeat("+", 16)
)

func TestHideSecretDataSameKeysDifferentValues(t *testing.T) {
	target, live, err := HideSecretData(
		createSecret(map[string]string{"key1": "test", "key2": "test"}),
		createSecret(map[string]string{"key1": "test-1", "key2": "test-1"}),
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"key1": replacement1, "key2": replacement1}, secretData(target))
	assert.Equal(t, map[string]any{"key1": replacement2, "key2": replacement2}, secretData(live))
}

func TestHideSecretDataSameKeysSameValues(t *testing.T) {
	target, live, err := HideSecretData(
		createSecret(map[string]string{"key1": "test", "key2": "test"}),
		createSecret(map[string]string{"key1": "test", "key2": "test"}),
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"key1": replacement1, "key2": replacement1}, secretData(target))
	assert.Equal(t, map[string]any{"key1": replacement1, "key2": replacement1}, secretData(live))
}

func TestHideSecretDataDifferentKeysDifferentValues(t *testing.T) {
	target, live, err := HideSecretData(
		createSecret(map[string]string{"key1": "test", "key2": "test"}),
		createSecret(map[string]string{"key2": "test-1", "key3": "test-1"}),
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"key1": replacement1, "key2": replacement1}, secretData(target))
	assert.Equal(t, map[string]any{"key2": replacement2, "key3": replacement1}, secretData(live))
}

func TestHideStringDataInInvalidSecret(t *testing.T) {
	liveUn := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name": "test-secret",
			},
			"type": "Opaque",
			"data": map[string]any{
				"key1": "a2V5MQ==",
				"key2": "a2V5MQ==",
			},
		},
	}
	targetUn := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name": "test-secret",
			},
			"type": "Opaque",
			"data": map[string]any{
				"key1": "a2V5MQ==",
				"key2": "a2V5Mg==",
				"key3": false,
			},
			"stringData": map[string]any{
				"key4": "key4",
				"key5": 5,
			},
		},
	}

	liveUn = remarshal(liveUn, applyOptions(diffOptionsForTest()))
	targetUn = remarshal(targetUn, applyOptions(diffOptionsForTest()))

	target, live, err := HideSecretData(targetUn, liveUn, nil)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"key1": replacement1, "key2": replacement2}, secretData(live))
	assert.Equal(t, map[string]any{"key1": replacement1, "key2": replacement1, "key3": replacement1, "key4": replacement1, "key5": replacement1}, secretData(target))
}

// stringData in secrets should be normalized even if it is invalid
func TestNormalizeSecret(t *testing.T) {
	tests := []struct {
		testname   string
		data       map[string]any
		stringData map[string]any
	}{
		{
			testname: "Valid secret",
			data: map[string]any{
				"key1": "key1",
			},
			stringData: map[string]any{
				"key2": "a2V5Mg==",
			},
		},
		{
			testname: "Invalid secret",
			data: map[string]any{
				"key1": "key1",
				"key2": 2,
			},
			stringData: map[string]any{
				"key3": "key3",
				"key4": nil,
			},
		},
		{
			testname: "Invalid secret with stringData only",
			data:     nil,
			stringData: map[string]any{
				"key3": "key3",
				"key4": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			un := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name": "test-secret",
					},
					"type":       "Opaque",
					"data":       tt.data,
					"stringData": tt.stringData,
				},
			}
			un = remarshal(un, applyOptions(diffOptionsForTest()))

			NormalizeSecret(un)

			_, found, _ := unstructured.NestedMap(un.Object, "stringData")
			assert.False(t, found)

			data, found, _ := unstructured.NestedMap(un.Object, "data")
			assert.True(t, found)

			// check all secret keys are found under data in normalized secret
			for _, obj := range []map[string]any{tt.data, tt.stringData} {
				if obj == nil {
					continue
				}
				for k := range obj {
					_, ok := data[k]
					assert.True(t, ok)
				}
			}
		})
	}
}

func TestHideSecretAnnotations(t *testing.T) {
	tests := []struct {
		name           string
		hideAnnots     map[string]bool
		annots         map[string]any
		expectedAnnots map[string]any
		targetNil      bool
	}{
		{
			name:           "no hidden annotations",
			hideAnnots:     nil,
			annots:         map[string]any{"token/value": "secret", "key": "secret-key", "app": "test"},
			expectedAnnots: map[string]any{"token/value": "secret", "key": "secret-key", "app": "test"},
		},
		{
			name:           "hide annotations",
			hideAnnots:     map[string]bool{"token/value": true, "key": true},
			annots:         map[string]any{"token/value": "secret", "key": "secret-key", "app": "test"},
			expectedAnnots: map[string]any{"token/value": replacement1, "key": replacement1, "app": "test"},
		},
		{
			name:       "hide annotations in last-applied-config",
			hideAnnots: map[string]bool{"token/value": true, "key": true},
			annots: map[string]any{
				"token/value": "secret",
				"app":         "test",
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","kind":"Secret","metadata":{"annotations":{"app":"test","token/value":"secret","key":"secret-key"},"labels":{"app.kubernetes.io/instance":"test"},"name":"my-secret","namespace":"default"},"type":"Opaque"}`,
			},
			expectedAnnots: map[string]any{
				"token/value": replacement1,
				"app":         "test",
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","kind":"Secret","metadata":{"annotations":{"app":"test","key":"++++++++","token/value":"++++++++"},"labels":{"app.kubernetes.io/instance":"test"},"name":"my-secret","namespace":"default"},"type":"Opaque"}`,
			},
			targetNil: true,
		},
		{
			name:       "special case: hide last-applied-config annotation",
			hideAnnots: map[string]bool{"kubectl.kubernetes.io/last-applied-configuration": true},
			annots: map[string]any{
				"token/value": replacement1,
				"app":         "test",
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","kind":"Secret","metadata":{"annotations":{"app":"test","token/value":"secret","key":"secret-key"},"labels":{"app.kubernetes.io/instance":"test"},"name":"my-secret","namespace":"default"},"type":"Opaque"}`,
			},
			expectedAnnots: map[string]any{
				"app": "test",
				"kubectl.kubernetes.io/last-applied-configuration": replacement1,
			},
			targetNil: true,
		},
		{
			name:           "hide annotations for malformed annotations",
			hideAnnots:     map[string]bool{"token/value": true, "key": true},
			annots:         map[string]any{"token/value": 0, "key": "secret", "app": true},
			expectedAnnots: map[string]any{"token/value": replacement1, "key": replacement1, "app": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unSecret := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name":        "test-secret",
						"annotations": tt.annots,
					},
					"type": "Opaque",
				},
			}

			liveUn := remarshal(unSecret, applyOptions(diffOptionsForTest()))
			targetUn := remarshal(unSecret, applyOptions(diffOptionsForTest()))

			if tt.targetNil {
				targetUn = nil
			}

			target, live, err := HideSecretData(targetUn, liveUn, tt.hideAnnots)
			require.NoError(t, err)

			// verify configured annotations are hidden
			for _, obj := range []*unstructured.Unstructured{target, live} {
				if obj != nil {
					annots, _, _ := unstructured.NestedMap(obj.Object, "metadata", "annotations")
					for ek, ev := range tt.expectedAnnots {
						v, found := annots[ek]
						assert.True(t, found)
						assert.Equal(t, ev, v)
					}
				}
			}
		})
	}
}

func TestHideSecretAnnotationsPreserveDifference(t *testing.T) {
	hideAnnots := map[string]bool{"token/value": true}

	liveUn := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":        "test-secret",
				"annotations": map[string]any{"token/value": "secret", "app": "test"},
			},
			"type": "Opaque",
		},
	}
	targetUn := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":        "test-secret",
				"annotations": map[string]any{"token/value": "new-secret", "app": "test"},
			},
			"type": "Opaque",
		},
	}

	liveUn = remarshal(liveUn, applyOptions(diffOptionsForTest()))
	targetUn = remarshal(targetUn, applyOptions(diffOptionsForTest()))

	target, live, err := HideSecretData(targetUn, liveUn, hideAnnots)
	require.NoError(t, err)

	liveAnnots := live.GetAnnotations()
	v, found := liveAnnots["token/value"]
	assert.True(t, found)
	assert.Equal(t, replacement2, v)

	targetAnnots := target.GetAnnotations()
	v, found = targetAnnots["token/value"]
	assert.True(t, found)
	assert.Equal(t, replacement1, v)
}

func getTargetSecretJsonBytes() []byte {
	return []byte(`
{
    "apiVersion": "v1",
    "kind": "Secret",
    "type": "kubernetes.io/service-account-token",
    "metadata": {
        "annotations": {
            "kubernetes.io/service-account.name": "default"
        },
        "labels": {
            "app.kubernetes.io/instance": "empty-secret"
        },
        "name": "an-empty-secret",
        "namespace": "default"
    },
	"data": {}
}`)
}

func getLiveSecretJsonBytes() []byte {
	return []byte(`
{
    "kind": "Secret",
    "apiVersion": "v1",
    "type": "kubernetes.io/service-account-token",
    "metadata": {
        "annotations": {
            "kubernetes.io/service-account.name": "default",
            "kubernetes.io/service-account.uid": "78688180-d432-4ee8-939d-382b015a6b13"
        },
        "creationTimestamp": "2021-10-27T19:09:22Z",
        "labels": {
            "app.kubernetes.io/instance": "empty-secret"
        },
        "name": "an-empty-secret",
        "namespace": "default",
        "resourceVersion": "2329692",
        "uid": "2e98590d-a699-4281-89d5-aa94dfc1d7d7"
    },
    "data": {
        "namespace": "ZGVmYXVsdA==",
        "token": "ZGVmYXVsdAcb=="
    }
}`)
}

func bytesToUnstructured(t *testing.T, jsonBytes []byte) *unstructured.Unstructured {
	t.Helper()
	var jsonMap map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &jsonMap))
	return &unstructured.Unstructured{
		Object: jsonMap,
	}
}

func TestHideSecretDataHandleEmptySecret(t *testing.T) {
	// given
	targetSecret := bytesToUnstructured(t, getTargetSecretJsonBytes())
	liveSecret := bytesToUnstructured(t, getLiveSecretJsonBytes())

	// when
	target, live, err := HideSecretData(targetSecret, liveSecret, nil)

	// then
	require.NoError(t, err)
	assert.NotNil(t, target)
	assert.NotNil(t, live)
	assert.Nil(t, target.Object["data"])
	assert.Equal(t, map[string]any{"namespace": "++++++++", "token": "++++++++"}, secretData(live))
}

func TestHideSecretDataLastAppliedConfig(t *testing.T) {
	lastAppliedSecret := createSecret(map[string]string{"key1": "test1"})
	targetSecret := createSecret(map[string]string{"key1": "test2"})
	liveSecret := createSecret(map[string]string{"key1": "test3"})
	lastAppliedStr, err := json.Marshal(lastAppliedSecret)
	require.NoError(t, err)
	liveSecret.SetAnnotations(map[string]string{corev1.LastAppliedConfigAnnotation: string(lastAppliedStr)})

	target, live, err := HideSecretData(targetSecret, liveSecret, nil)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(live.GetAnnotations()[corev1.LastAppliedConfigAnnotation]), &lastAppliedSecret)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"key1": replacement1}, secretData(target))
	assert.Equal(t, map[string]any{"key1": replacement2}, secretData(live))
	assert.Equal(t, map[string]any{"key1": replacement3}, secretData(lastAppliedSecret))
}

func TestRemarshal(t *testing.T) {
	manifest := []byte(`
apiVersion: v1
kind: ServiceAccount
imagePullSecrets: []
metadata:
  name: my-sa
`)
	var un unstructured.Unstructured
	require.NoError(t, yaml.Unmarshal(manifest, &un))
	newUn := remarshal(&un, applyOptions(diffOptionsForTest()))
	_, ok := newUn.Object["imagePullSecrets"]
	assert.False(t, ok)
	metadata := newUn.Object["metadata"].(map[string]any)
	_, ok = metadata["creationTimestamp"]
	assert.False(t, ok)
}

func TestRemarshalResources(t *testing.T) {
	getRequests := func(un *unstructured.Unstructured) map[string]any {
		return un.Object["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)["resources"].(map[string]any)["requests"].(map[string]any)
	}

	setRequests := func(un *unstructured.Unstructured, requests map[string]any) {
		un.Object["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)["resources"].(map[string]any)["requests"] = requests
	}

	manifest := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - image: nginx:1.7.9
    name: nginx
    resources:
      requests:
        cpu: 0.2
`)
	un := unstructured.Unstructured{}
	require.NoError(t, yaml.Unmarshal(manifest, &un))

	testCases := []struct {
		name        string
		cpu         any
		expectedCPU any
	}{
		{"from float", 0.2, "200m"},
		{"from float64", float64(0.2), "200m"},
		{"from string", "0.2", "200m"},
		{"from invalid", "invalid", "invalid"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setRequests(&un, map[string]any{"cpu": tc.cpu})
			newUn := remarshal(&un, applyOptions(diffOptionsForTest()))
			requestsAfter := getRequests(newUn)
			assert.Equal(t, tc.expectedCPU, requestsAfter["cpu"])
		})
	}
}

func ExampleDiff() {
	expectedResource := unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(`
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - image: nginx:1.7.9
    name: nginx
    resources:
      requests:
        cpu: 0.2
`), &expectedResource); err != nil {
		panic(err)
	}

	liveResource := unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(`
apiVersion: v1
kind: Pod
metadata:
  name: my-pod-123
  creationTimestamp: "2020-03-30T21:34:59Z"
  labels:
    pod-template-hash: 84bf9649fd
  name: argo-cd-cli-84bf9649fd-tm59q
  resourceVersion: "233081332"
  uid: 9a5ae31a-eed2-4f82-81fe-833799c54f99
spec:
  containers:
  - image: nginx:1.7.9
    name: nginx
    resources:
      requests:
        cpu: 0.1
`), &liveResource); err != nil {
		panic(err)
	}
	diff, err := Diff(&expectedResource, &liveResource, diffOptionsForTest()...)
	if err != nil {
		panic(err)
	}
	if diff.Modified {
		fmt.Println("Resources are different")
	}
}

func diffOptionsForTest() []Option {
	return []Option{
		WithLogr(textlogger.NewLogger(textlogger.NewConfig())),
		IgnoreAggregatedRoles(false),
	}
}

func YamlToSvc(t *testing.T, y []byte) *corev1.Service {
	t.Helper()
	svc := corev1.Service{}
	require.NoErrorf(t, yaml.Unmarshal(y, &svc), "error unmarshaling service bytes")
	return &svc
}

func YamlToDeploy(t *testing.T, y []byte) *appsv1.Deployment {
	t.Helper()
	deploy := appsv1.Deployment{}
	require.NoErrorf(t, yaml.Unmarshal(y, &deploy), "error unmarshaling deployment bytes")
	return &deploy
}

func StrToUnstructured(yamlStr string) *unstructured.Unstructured {
	obj := make(map[string]any)
	err := yaml.Unmarshal([]byte(yamlStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}
