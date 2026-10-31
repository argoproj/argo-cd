package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/scheme"
)

func TestFoldSecretStringData(t *testing.T) {
	t.Run("stringData is base64 encoded into data and wins over data", func(t *testing.T) {
		secret := toUnstructured(t, `
apiVersion: v1
kind: Secret
metadata:
  name: s
data:
  password: ZnJvbS1kYXRh
  other: b3RoZXI=
stringData:
  password: from-stringData
  username: app
`)
		foldSecretStringData(secret)
		_, hasStringData := secret.Object["stringData"]
		assert.False(t, hasStringData)
		data, _, err := unstructured.NestedStringMap(secret.Object, "data")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"password": "ZnJvbS1zdHJpbmdEYXRh", // "from-stringData"
			"username": "YXBw",                 // "app"
			"other":    "b3RoZXI=",
		}, data)
	})
	t.Run("non-Secret kinds are untouched", func(t *testing.T) {
		cm := toUnstructured(t, `
apiVersion: v1
kind: ConfigMap
metadata:
  name: c
stringData:
  key: value
`)
		foldSecretStringData(cm)
		_, hasStringData := cm.Object["stringData"]
		assert.True(t, hasStringData)
	})
}

func TestRemovePath(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"replicas": int64(1),
			"items":    []any{"a", "b", "c"},
		},
	}
	removePath(obj, []any{"spec", "replicas"})
	removePath(obj, []any{"spec", "items", 1})
	removePath(obj, []any{"spec", "missing", "deeper"})
	removePath(obj, []any{"spec", "items", 7})
	assert.Equal(t, map[string]any{"spec": map[string]any{"items": []any{"a", "c"}}}, obj)
}

func TestComparePaths(t *testing.T) {
	assert.Negative(t, comparePaths([]any{"spec"}, []any{"spec", "replicas"}), "parent sorts before child")
	assert.Negative(t, comparePaths([]any{"spec", "items", 0}, []any{"spec", "items", 1}), "lower index first")
	assert.Negative(t, comparePaths([]any{"spec", "items", 2}, []any{"spec", "items", 10}), "indexes compare numerically")
	assert.Zero(t, comparePaths([]any{"a", 1}, []any{"a", 1}))
	assert.Positive(t, comparePaths([]any{"b"}, []any{"a"}))
}

func deploymentSchema(t *testing.T) fieldSchema {
	t.Helper()
	return staticFieldSchema(t, deploymentGVK)
}

func staticFieldSchema(t *testing.T, gvk schema.GroupVersionKind) fieldSchema {
	t.Helper()
	pt := scheme.StaticParser().Type(staticTypeName(gvk))
	require.True(t, pt.IsValid())
	return newFieldSchema(&pt)
}

func TestBuildApplyTarget(t *testing.T) {
	deployment := func(t *testing.T, podSpecYAML string) *unstructured.Unstructured {
		t.Helper()
		return toUnstructured(t, `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d
  namespace: default
spec:
  replicas: 1
  template:
    spec:
`+podSpecYAML)
	}
	sch := deploymentSchema(t)
	containersOf := func(t *testing.T, u *unstructured.Unstructured) []any {
		t.Helper()
		containers, _, err := unstructured.NestedSlice(u.Object, "spec", "template", "spec", "containers")
		require.NoError(t, err)
		return containers
	}

	t.Run("keyed item that omits a defaulted key field is matched by the default", func(t *testing.T) {
		// Service ports are keyed by (port, protocol); manifests omit protocol
		// and the server defaults it to TCP. The live-only nodePort must land
		// in git's port, not in a duplicate item.
		live := toUnstructured(t, `
apiVersion: v1
kind: Service
metadata: {name: s, namespace: default}
spec:
  ports:
  - {name: http, port: 80, targetPort: 8080, protocol: TCP, nodePort: 31234}
`)
		target := toUnstructured(t, `
apiVersion: v1
kind: Service
metadata: {name: s, namespace: default}
spec:
  ports:
  - {name: http, port: 80, targetPort: 8080}
`)
		got, err := buildApplyTarget(live, target, [][]any{{"spec", "ports", 0, "nodePort"}}, nil, staticFieldSchema(t, serviceGVK))
		require.NoError(t, err)
		ports, _, err := unstructured.NestedSlice(got.Object, "spec", "ports")
		require.NoError(t, err)
		require.Len(t, ports, 1, "no duplicate port item")
		assert.Equal(t, map[string]any{"name": "http", "port": int64(80), "targetPort": int64(8080), "nodePort": int64(31234)}, ports[0])
	})

	t.Run("field inside an undeclared item of a list without keys carries the whole item", func(t *testing.T) {
		// tolerations is atomic: a partial item would be applied as is and
		// rejected (a toleration with a value but no key).
		live := deployment(t, `      containers: [{name: web, image: nginx}]
      tolerations:
      - {key: a, operator: Exists}
      - {key: injected, operator: Equal, value: v, effect: NoSchedule}
`)
		target := deployment(t, `      containers: [{name: web, image: nginx}]
      tolerations:
      - {key: a, operator: Exists}
`)
		got, err := buildApplyTarget(live, target, [][]any{{"spec", "template", "spec", "tolerations", 1, "value"}}, nil, sch)
		require.NoError(t, err)
		tolerations, _, err := unstructured.NestedSlice(got.Object, "spec", "template", "spec", "tolerations")
		require.NoError(t, err)
		assert.Equal(t, []any{
			map[string]any{"key": "a", "operator": "Exists"},
			map[string]any{"key": "injected", "operator": "Equal", "value": "v", "effect": "NoSchedule"},
		}, tolerations)
	})

	t.Run("server-populated metadata is never carried", func(t *testing.T) {
		live := deployment(t, "      containers: [{name: web, image: nginx:live}]\n")
		live.SetLabels(map[string]string{"owner": "live"})
		live.SetUID("1234")
		live.SetResourceVersion("99")
		live.SetGeneration(3)
		require.NoError(t, unstructured.SetNestedSlice(live.Object, []any{map[string]any{"manager": "x", "operation": "Update"}}, "metadata", "managedFields"))
		target := deployment(t, "      containers: [{name: web, image: nginx:git}]\n")

		got, err := buildApplyTarget(live, target, [][]any{{"metadata"}, {"metadata", "uid"}}, [][]any{{"metadata"}}, sch)
		require.NoError(t, err)

		assert.Equal(t, "d", got.GetName())
		assert.Equal(t, "default", got.GetNamespace())
		assert.Equal(t, map[string]string{"owner": "live"}, got.GetLabels())
		meta := got.Object["metadata"].(map[string]any)
		for _, field := range serverMetadataFields {
			assert.NotContains(t, meta, field)
		}
	})

	t.Run("ignored target path is not asserted and live value is carried", func(t *testing.T) {
		live := deployment(t, "      containers: [{name: web, image: nginx:live}]\n")
		require.NoError(t, unstructured.SetNestedField(live.Object, int64(5), "spec", "replicas"))
		target := deployment(t, "      containers: [{name: web, image: nginx:git}]\n")

		got, err := buildApplyTarget(live, target, [][]any{{"spec", "replicas"}}, [][]any{{"spec", "replicas"}}, sch)
		require.NoError(t, err)

		replicas, _, err := unstructured.NestedInt64(got.Object, "spec", "replicas")
		require.NoError(t, err)
		assert.Equal(t, int64(5), replicas)
		assert.Equal(t, "nginx:git", containersOf(t, got)[0].(map[string]any)["image"], "non-ignored fields come from git")
	})

	t.Run("git wins when a rule removed the path only from live", func(t *testing.T) {
		live := deployment(t, "      containers: [{name: web, image: nginx:live}]\n")
		target := deployment(t, "      containers: [{name: web, image: nginx:git}]\n")

		got, err := buildApplyTarget(live, target, [][]any{{"spec", "template", "spec", "containers", 0, "image"}}, nil, sch)
		require.NoError(t, err)

		assert.Equal(t, "nginx:git", containersOf(t, got)[0].(map[string]any)["image"])
	})

	t.Run("live-only keyed list item is inserted at its live position", func(t *testing.T) {
		live := deployment(t, "      containers: [{name: sidecar, image: proxy:1}, {name: web, image: nginx:1}]\n")
		target := deployment(t, "      containers: [{name: web, image: nginx:1}]\n")

		got, err := buildApplyTarget(live, target, [][]any{{"spec", "template", "spec", "containers", 0}}, nil, sch)
		require.NoError(t, err)

		assert.Equal(t, []string{"sidecar", "web"}, names(containersOf(t, got)))
	})

	t.Run("field inside a keyed list item is matched by key, not position", func(t *testing.T) {
		live := deployment(t, "      containers: [{name: sidecar, image: proxy:1}, {name: web, image: nginx:live}]\n")
		target := deployment(t, "      containers: [{name: web, image: nginx:git}, {name: sidecar, image: proxy:1}]\n")

		// live path is containers[1].image (web); in git web is containers[0].
		got, err := buildApplyTarget(live, target,
			[][]any{{"spec", "template", "spec", "containers", 1, "image"}},
			[][]any{{"spec", "template", "spec", "containers", 0, "image"}}, sch)
		require.NoError(t, err)

		containers := containersOf(t, got)
		require.Equal(t, []string{"web", "sidecar"}, names(containers))
		assert.Equal(t, "nginx:live", containers[0].(map[string]any)["image"])
	})

	t.Run("atomic map absent from git is not created", func(t *testing.T) {
		live := deployment(t, "      nodeSelector: {a: \"1\", b: \"2\"}\n      containers: [{name: web, image: nginx:1}]\n")
		target := deployment(t, "      containers: [{name: web, image: nginx:1}]\n")

		got, err := buildApplyTarget(live, target, [][]any{{"spec", "template", "spec", "nodeSelector", "b"}}, nil, sch)
		require.NoError(t, err)

		_, found, _ := unstructured.NestedMap(got.Object, "spec", "template", "spec", "nodeSelector")
		assert.False(t, found)
	})

	t.Run("granular map absent from git is created for the carried key", func(t *testing.T) {
		live := deployment(t, "      containers: [{name: web, image: nginx:1}]\n")
		require.NoError(t, unstructured.SetNestedStringMap(live.Object, map[string]string{"a": "1", "b": "2"}, "metadata", "labels"))
		target := deployment(t, "      containers: [{name: web, image: nginx:1}]\n")

		got, err := buildApplyTarget(live, target, [][]any{{"metadata", "labels", "b"}}, nil, sch)
		require.NoError(t, err)

		assert.Equal(t, map[string]string{"b": "2"}, got.GetLabels())
	})

	t.Run("status is never carried", func(t *testing.T) {
		live := deployment(t, "      containers: [{name: web, image: nginx:1}]\n")
		require.NoError(t, unstructured.SetNestedField(live.Object, int64(3), "status", "replicas"))
		target := deployment(t, "      containers: [{name: web, image: nginx:1}]\n")

		got, err := buildApplyTarget(live, target, [][]any{{"status", "replicas"}}, nil, sch)
		require.NoError(t, err)

		_, found := got.Object["status"]
		assert.False(t, found)
	})

	t.Run("duplicate merge keys match the first item and do not error", func(t *testing.T) {
		// A Deployment created with client-side apply can hold two env entries
		// with one name. The typed conversion used to fail on it
		// (https://github.com/argoproj/argo-cd/issues/18402); matching by key
		// takes the first item and never inserts a third.
		live := deployment(t, "      containers: [{name: web, image: nginx, env: [{name: DUP, value: live-1}, {name: DUP, value: live-2}]}]\n")
		target := deployment(t, "      containers: [{name: web, image: nginx, env: [{name: DUP}, {name: DUP, value: git-2}]}]\n")

		got, err := buildApplyTarget(live, target, [][]any{{"spec", "template", "spec", "containers", 0, "env", 1, "value"}}, nil, sch)
		require.NoError(t, err)

		assert.Equal(t, []any{
			map[string]any{"name": "DUP", "value": "live-2"},
			map[string]any{"name": "DUP", "value": "git-2"},
		}, containersOf(t, got)[0].(map[string]any)["env"])
	})

	t.Run("a whole-object path carries nothing", func(t *testing.T) {
		// jsonPointers: [""] or jqPathExpressions: ["."] resolve to the empty
		// path on both sides. Removing it would empty the manifest; carrying it
		// would replace the manifest with live. Both are skipped.
		live := deployment(t, "      replicas: 2\n")
		target := deployment(t, "      replicas: 1\n")
		applied, err := buildApplyTarget(live, target, [][]any{{}}, [][]any{{}}, deploymentSchema(t))
		require.NoError(t, err)
		assert.Equal(t, target.Object, applied.Object)
	})
}

func TestFieldSchema(t *testing.T) {
	t.Run("static schema of a built-in", func(t *testing.T) {
		podSpec := deploymentSchema(t).child("spec").child("template").child("spec")
		assert.Equal(t, []string{"name"}, podSpec.child("containers").listKeys())
		assert.False(t, podSpec.child("containers").atomicList())
		assert.Equal(t, []string{"name"}, podSpec.child("containers").item().child("env").listKeys())
		assert.True(t, podSpec.child("tolerations").atomicList())
		assert.Nil(t, podSpec.child("tolerations").listKeys())
		assert.True(t, podSpec.child("nodeSelector").atomicMap())
		assert.False(t, deploymentSchema(t).child("metadata").child("labels").atomicMap())
	})
	t.Run("ingress paths are atomic", func(t *testing.T) {
		pt := scheme.StaticParser().Type(staticTypeName(ingressGVK))
		require.True(t, pt.IsValid())
		paths := newFieldSchema(&pt).child("spec").child("rules").item().child("http").child("paths")
		assert.True(t, paths.atomicList())
		assert.Nil(t, paths.listKeys())
	})
	t.Run("deduced type treats lists as atomic and maps as granular", func(t *testing.T) {
		sch := newFieldSchema(&typed.DeducedParseableType)
		assert.True(t, sch.child("spec").child("items").atomicList())
		assert.Nil(t, sch.child("spec").child("items").listKeys())
		assert.False(t, sch.child("spec").atomicMap())
	})
	t.Run("unknown type behaves like the deduced type", func(t *testing.T) {
		sch := newFieldSchema(nil)
		assert.True(t, sch.child("spec").child("items").atomicList())
		assert.Nil(t, sch.child("spec").child("items").listKeys())
		assert.False(t, sch.child("spec").atomicMap())
	})
	t.Run("a field the schema does not declare behaves like the deduced type", func(t *testing.T) {
		// A cluster newer than the static schema has fields the schema lacks
		// (https://github.com/argoproj/argo-cd/issues/18778): no error, lists
		// atomic, maps granular, as the API server deduces them.
		future := deploymentSchema(t).child("spec").child("futureField")
		assert.True(t, future.child("items").atomicList())
		assert.Nil(t, future.child("items").listKeys())
		assert.False(t, future.atomicMap())
		assert.False(t, future.child("nested").atomicMap())
	})
	t.Run("a preserve-unknown-fields subtree behaves like the deduced type", func(t *testing.T) {
		// helm.valuesObject on the Application CRD
		// (https://github.com/argoproj/argo-cd/issues/25199).
		pt := gvkParserFromSchema(t, applicationSchema).Type(applicationGVK)
		require.NotNil(t, pt)
		values := newFieldSchema(pt).child("spec").child("source").child("helm").child("valuesObject")
		assert.False(t, values.atomicMap())
		assert.False(t, values.child("workflow").atomicMap())
		assert.True(t, values.child("workflow").child("items").atomicList())
		assert.Nil(t, values.child("workflow").child("items").listKeys())
	})
}
