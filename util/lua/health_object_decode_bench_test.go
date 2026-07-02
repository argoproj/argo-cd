package lua

import (
	"encoding/json"
	"testing"

	lua "github.com/yuin/gopher-lua"
	corev1 "k8s.io/api/core/v1"
)

// realisticManagedFields mirrors metadata.managedFields on a live Pod: kube-controller-manager
// owns spec, kubelet owns status. Testdata omits this block, but every real object carries one.
const realisticManagedFields = `[
  {
    "manager": "kube-controller-manager",
    "operation": "Update",
    "apiVersion": "v1",
    "time": "2026-06-12T17:00:00Z",
    "fieldsType": "FieldsV1",
    "fieldsV1": {
      "f:metadata": {
        "f:labels": {".": {}, "f:app": {}},
        "f:ownerReferences": {
          ".": {},
          "k:{\"uid\":\"a1b2c3d4-0000-1111-2222-333344445555\"}": {}
        }
      },
      "f:spec": {
        "f:containers": {
          "k:{\"name\":\"app\"}": {
            ".": {},
            "f:image": {},
            "f:name": {},
            "f:resources": {
              ".": {},
              "f:limits": {".": {}, "f:cpu": {}, "f:memory": {}},
              "f:requests": {".": {}, "f:cpu": {}, "f:memory": {}}
            }
          }
        },
        "f:restartPolicy": {}
      }
    }
  },
  {
    "manager": "kubelet",
    "operation": "Update",
    "apiVersion": "v1",
    "time": "2026-06-12T17:00:05Z",
    "fieldsType": "FieldsV1",
    "subresource": "status",
    "fieldsV1": {
      "f:status": {
        "f:conditions": {
          "k:{\"type\":\"Ready\"}": {
            ".": {},
            "f:status": {},
            "f:type": {}
          }
        },
        "f:containerStatuses": {},
        "f:phase": {},
        "f:podIP": {}
      }
    }
  }
]`

func benchmarkHealthObject(b *testing.B) map[string]any {
	b.Helper()
	var managedFields []any
	if err := json.Unmarshal([]byte(realisticManagedFields), &managedFields); err != nil {
		b.Fatal(err)
	}
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      "app-abc123",
			"namespace": "default",
			"labels": map[string]any{
				"app": "demo",
			},
			"annotations": map[string]any{
				corev1.LastAppliedConfigAnnotation: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"app-abc123","namespace":"default"},"spec":{"containers":[{"name":"app","image":"demo:1"}]}}`,
				"example.com/health-signal":        "ok",
			},
			"managedFields": managedFields,
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "app",
					"image": "demo:1",
				},
			},
		},
		"status": map[string]any{
			"phase": "Running",
			"conditions": []any{
				map[string]any{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}
}

func BenchmarkHealthObjectDecode(b *testing.B) {
	obj := benchmarkHealthObject(b)
	L := lua.NewState()
	defer L.Close()

	b.Run("full", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			decodeValue(L, obj)
		}
	})

	b.Run("health", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			decodeHealthObject(L, obj)
		}
	})
}
