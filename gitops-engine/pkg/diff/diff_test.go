package diff

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/diff/testdata"
	openapi_v2 "github.com/google/gnostic/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/klog/v2/klogr"
	openapiproto "k8s.io/kube-openapi/pkg/util/proto"
	"sigs.k8s.io/yaml"
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
	err = os.WriteFile(targetFile, targetData, 0644)
	if err != nil {
		return nil, err
	}
	liveFile := filepath.Join(tempDir, fmt.Sprintf("%s-live.yaml", name))
	liveData := []byte("")
	if live != nil {
		liveData, err = yaml.Marshal(live)
		if err != nil {
			return nil, err
		}
	}
	err = os.WriteFile(liveFile, liveData, 0644)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("diff", liveFile, targetFile)
	return cmd.Output()
}

func toUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	uObj, err := runtime.NewTestUnstructuredConverter(equality.Semantic).ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: uObj}, nil
}

func mustToUnstructured(obj interface{}) *unstructured.Unstructured {
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
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "demo",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "demo",
							Image: "gcr.io/kuar-demo/kuard-amd64:1",
							Ports: []v1.ContainerPort{
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
	if !assert.NoError(t, unstructured.SetNestedField(leftUn.Object, "badValue", "spec", "revisionHistoryLimit")) {
		return
	}

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
		if !assert.NoError(t, unstructured.SetNestedField(rightUn.Object, "3", "spec", "revisionHistoryLimit")) {
			return
		}

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
	assert.NoError(t, err)
	assert.False(t, diffRes.Modified)

	diffRes = diff(t, resource, nil, diffOptionsForTest()...)
	assert.True(t, diffRes.Modified)
	diffRes, err = TwoWayDiff(resource, nil)
	assert.NoError(t, err)
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
	liveDep.Annotations[v1.LastAppliedConfigAnnotation] = string(configBytes)
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
	delete(liveDep.Annotations, v1.LastAppliedConfigAnnotation)
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
	assert.Nil(t, err)

	assert.True(t, strings.Contains(ascii, "invalidKey"))
	if ascii != "" {
		t.Log(ascii)
	}
}

func TestRemoveNamespaceAnnotation(t *testing.T) {
	obj := removeNamespaceAnnotation(&unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "default",
		},
	}})
	assert.Equal(t, "", obj.GetNamespace())

	obj = removeNamespaceAnnotation(&unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":        "test",
			"namespace":   "default",
			"annotations": make(map[string]interface{}),
		},
	}})
	assert.Equal(t, "", obj.GetNamespace())
	assert.Nil(t, obj.GetAnnotations())

	obj = removeNamespaceAnnotation(&unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
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
	configData := configUn.Object["data"].(map[string]interface{})
	liveData := liveUn.Object["data"].(map[string]interface{})
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
	document := &openapi_v2.Document{}
	err := proto.Unmarshal(testdata.OpenAPIV2Doc, document)
	if err != nil {
		t.Fatalf("error unmarshaling openapi doc: %s", err)
	}
	models, err := openapiproto.NewOpenAPIData(document)
	if err != nil {
		t.Fatalf("error building openapi data: %s", err)
	}

	gvkParser, err := managedfields.NewGVKParser(models, false)
	if err != nil {
		t.Fatalf("error building gvkParser: %s", err)
	}
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

func secretData(obj *unstructured.Unstructured) map[string]interface{} {
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
		createSecret(map[string]string{"key1": "test-1", "key2": "test-1"}))
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{"key1": replacement1, "key2": replacement1}, secretData(target))
	assert.Equal(t, map[string]interface{}{"key1": replacement2, "key2": replacement2}, secretData(live))
}

func TestHideSecretDataSameKeysSameValues(t *testing.T) {
	target, live, err := HideSecretData(
		createSecret(map[string]string{"key1": "test", "key2": "test"}),
		createSecret(map[string]string{"key1": "test", "key2": "test"}))
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{"key1": replacement1, "key2": replacement1}, secretData(target))
	assert.Equal(t, map[string]interface{}{"key1": replacement1, "key2": replacement1}, secretData(live))
}

func TestHideSecretDataDifferentKeysDifferentValues(t *testing.T) {
	target, live, err := HideSecretData(
		createSecret(map[string]string{"key1": "test", "key2": "test"}),
		createSecret(map[string]string{"key2": "test-1", "key3": "test-1"}))
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{"key1": replacement1, "key2": replacement1}, secretData(target))
	assert.Equal(t, map[string]interface{}{"key2": replacement2, "key3": replacement1}, secretData(live))
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
	var jsonMap map[string]interface{}
	err := json.Unmarshal(jsonBytes, &jsonMap)
	if err != nil {
		t.Fatal(err)
	}
	return &unstructured.Unstructured{
		Object: jsonMap,
	}
}

func TestHideSecretDataHandleEmptySecret(t *testing.T) {
	// given
	targetSecret := bytesToUnstructured(t, getTargetSecretJsonBytes())
	liveSecret := bytesToUnstructured(t, getLiveSecretJsonBytes())

	// when
	target, live, err := HideSecretData(targetSecret, liveSecret)

	// then
	assert.NoError(t, err)
	assert.NotNil(t, target)
	assert.NotNil(t, live)
	assert.Equal(t, nil, target.Object["data"])
	assert.Equal(t, map[string]interface{}{"namespace": "++++++++", "token": "++++++++"}, secretData(live))
}

func TestHideSecretDataLastAppliedConfig(t *testing.T) {
	lastAppliedSecret := createSecret(map[string]string{"key1": "test1"})
	targetSecret := createSecret(map[string]string{"key1": "test2"})
	liveSecret := createSecret(map[string]string{"key1": "test3"})
	lastAppliedStr, err := json.Marshal(lastAppliedSecret)
	require.NoError(t, err)
	liveSecret.SetAnnotations(map[string]string{corev1.LastAppliedConfigAnnotation: string(lastAppliedStr)})

	target, live, err := HideSecretData(targetSecret, liveSecret)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(live.GetAnnotations()[corev1.LastAppliedConfigAnnotation]), &lastAppliedSecret)
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{"key1": replacement1}, secretData(target))
	assert.Equal(t, map[string]interface{}{"key1": replacement2}, secretData(live))
	assert.Equal(t, map[string]interface{}{"key1": replacement3}, secretData(lastAppliedSecret))

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
	err := yaml.Unmarshal(manifest, &un)
	assert.NoError(t, err)
	newUn := remarshal(&un, applyOptions(diffOptionsForTest()))
	_, ok := newUn.Object["imagePullSecrets"]
	assert.False(t, ok)
	metadata := newUn.Object["metadata"].(map[string]interface{})
	_, ok = metadata["creationTimestamp"]
	assert.False(t, ok)
}

func TestRemarshalResources(t *testing.T) {
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
	err := yaml.Unmarshal(manifest, &un)
	assert.NoError(t, err)
	requestsBefore := un.Object["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["resources"].(map[string]interface{})["requests"].(map[string]interface{})
	t.Log(requestsBefore)
	newUn := remarshal(&un, applyOptions(diffOptionsForTest()))
	requestsAfter := newUn.Object["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["resources"].(map[string]interface{})["requests"].(map[string]interface{})
	t.Log(requestsAfter)
	assert.Equal(t, float64(0.2), requestsBefore["cpu"])
	assert.Equal(t, "200m", requestsAfter["cpu"])
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
		WithLogr(klogr.New()),
		IgnoreAggregatedRoles(false),
	}
}

func YamlToSvc(t *testing.T, y []byte) *corev1.Service {
	t.Helper()
	svc := corev1.Service{}
	err := yaml.Unmarshal(y, &svc)
	if err != nil {
		t.Fatalf("error unmarshaling service bytes: %s", err)
	}
	return &svc
}

func YamlToDeploy(t *testing.T, y []byte) *appsv1.Deployment {
	t.Helper()
	deploy := appsv1.Deployment{}
	err := yaml.Unmarshal(y, &deploy)
	if err != nil {
		t.Fatalf("error unmarshaling deployment bytes: %s", err)
	}
	return &deploy
}

func StrToUnstructured(yamlStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yamlStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}
