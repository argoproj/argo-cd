package cache

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func BenchmarkManifestSerialization(b *testing.B) {
	storageTypes := []ManifestStorageType{
		ManifestStorageJSON,
		ManifestStorageJSONIter,
		ManifestStorageMsgPack,
	}

	for _, testCase := range []struct {
		name string
		un   *unstructured.Unstructured
	}{
		{name: "typical-deployment", un: typicalDeploymentManifest()},
		{name: "large-configmap", un: largeConfigMapManifest()},
	} {
		jsonData, err := serializeManifestObject(testCase.un.Object, ManifestStorageJSON)
		if err != nil {
			b.Fatal(err)
		}

		for _, storageType := range storageTypes {
			b.Run(testCase.name+"/"+string(storageType)+"/write", func(b *testing.B) {
				b.ReportAllocs()
				b.ReportMetric(float64(len(jsonData)), "json-bytes")
				for range b.N {
					resource := &Resource{}
					if err := resource.SetManifestWithCodec(testCase.un, storageType, ManifestCompressionNone); err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run(testCase.name+"/"+string(storageType)+"/read", func(b *testing.B) {
				resource := &Resource{}
				if err := resource.SetManifestWithCodec(testCase.un, storageType, ManifestCompressionNone); err != nil {
					b.Fatal(err)
				}

				b.ReportAllocs()
				b.ReportMetric(float64(len(jsonData)), "json-bytes")
				b.ResetTimer()
				for range b.N {
					manifest, err := resource.GetManifest()
					if err != nil {
						b.Fatal(err)
					}
					if manifest == nil {
						b.Fatal("expected manifest")
					}
				}
			})
		}
	}
}

func typicalDeploymentManifest() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "checkout-api",
			"namespace": "production",
			"labels": map[string]any{
				"app.kubernetes.io/name":       "checkout-api",
				"app.kubernetes.io/part-of":    "storefront",
				"app.kubernetes.io/managed-by": "argocd",
			},
			"annotations": map[string]any{
				"example.com/description": strings.Repeat("representative deployment metadata ", 20),
			},
		},
		"spec": map[string]any{
			"replicas": int64(3),
			"selector": map[string]any{"matchLabels": map[string]any{"app": "checkout-api"}},
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]any{"app": "checkout-api"}},
				"spec": map[string]any{"containers": []any{map[string]any{
					"name":      "api",
					"image":     "ghcr.io/example/checkout-api:v1.2.3",
					"ports":     []any{map[string]any{"containerPort": int64(8080)}},
					"resources": map[string]any{"requests": map[string]any{"cpu": "100m", "memory": "128Mi"}},
				}}},
			},
		},
	}}
}

func largeConfigMapManifest() *unstructured.Unstructured {
	data := make(map[string]any, 256)
	for i := range 256 {
		data[fmt.Sprintf("config-%03d", i)] = strings.Repeat("x", 4096)
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "large-application-config",
			"namespace": "production",
		},
		"data": data,
	}}
}
