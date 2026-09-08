package diff_test

import (
	"context"
	"testing"

	testutil "github.com/argoproj/argo-cd/v3/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argo "github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

func TestStateDiffWithAnnotationBasedIgnoreDifferences(t *testing.T) {
	t.Parallel()

	t.Run("ignores differences based on resource annotation", func(t *testing.T) {
		// given
		t.Parallel()
		desired := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "default",
					"annotations": map[string]interface{}{
						common.AnnotationKeyIgnoreDifferences: "/spec/replicas",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "test",
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "nginx",
									"image": "nginx:1.14.2",
								},
							},
						},
					},
				},
			},
		}

		live := desired.DeepCopy()
		// Live state has different replicas (should be ignored due to annotation)
		_ = unstructured.SetNestedField(live.Object, int64(5), "spec", "replicas")

		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{}, true, normalizers.IgnoreNormalizerOpts{}).
			WithTracking("", "").
			WithNoCache().
			Build()
		require.NoError(t, err)

		// when
		result, err := argo.StateDiff(context.Background(), live, desired, diffConfig)

		// then
		require.NoError(t, err)
		assert.False(t, result.Modified, "Deployment should not be modified because replicas is ignored via annotation")
	})

	t.Run("ignores multiple fields based on resource annotation", func(t *testing.T) {
		// given
		t.Parallel()
		desired := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "default",
					"annotations": map[string]interface{}{
						common.AnnotationKeyIgnoreDifferences: "/spec/replicas,/metadata/labels/version",
					},
					"labels": map[string]interface{}{
						"app":     "test",
						"version": "1.0",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "test",
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "nginx",
									"image": "nginx:1.14.2",
								},
							},
						},
					},
				},
			},
		}

		live := desired.DeepCopy()
		// Live state has different replicas and version label (both should be ignored)
		_ = unstructured.SetNestedField(live.Object, int64(5), "spec", "replicas")
		_ = unstructured.SetNestedField(live.Object, "2.0", "metadata", "labels", "version")

		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{}, true, normalizers.IgnoreNormalizerOpts{}).
			WithTracking("", "").
			WithNoCache().
			Build()
		require.NoError(t, err)
		// when
		result, err := argo.StateDiff(context.Background(), live, desired, diffConfig)

		// then
		require.NoError(t, err)
		assert.False(t, result.Modified, "Deployment should not be modified because both fields are ignored via annotation")
	})

	t.Run("merges annotation-based ignores with application-level ignores", func(t *testing.T) {
		// given
		t.Parallel()
		desired := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "default",
					"annotations": map[string]interface{}{
						// Annotation ignores replicas
						common.AnnotationKeyIgnoreDifferences: "/spec/replicas",
					},
					"labels": map[string]interface{}{
						"app":     "test",
						"version": "1.0",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "test",
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "nginx",
									"image": "nginx:1.14.2",
								},
							},
						},
					},
				},
			},
		}

		live := desired.DeepCopy()
		// Live state has different replicas and version label
		_ = unstructured.SetNestedField(live.Object, int64(5), "spec", "replicas")
		_ = unstructured.SetNestedField(live.Object, "2.0", "metadata", "labels", "version")

		// Application-level ignore for version label
		appIgnores := []v1alpha1.ResourceIgnoreDifferences{
			{
				Group:        "apps",
				Kind:         "Deployment",
				JSONPointers: []string{"/metadata/labels/version"},
			},
		}

		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(appIgnores, map[string]v1alpha1.ResourceOverride{}, true, normalizers.IgnoreNormalizerOpts{}).
			WithTracking("", "").
			WithNoCache().
			Build()
		require.NoError(t, err)

		// when
		result, err := argo.StateDiff(context.Background(), live, desired, diffConfig)

		// then
		require.NoError(t, err)
		assert.False(t, result.Modified, "Deployment should not be modified because both annotation-based and app-level ignores apply")
	})

	t.Run("detects differences when annotation does not cover changed field", func(t *testing.T) {
		// given
		t.Parallel()
		desired := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "default",
					"annotations": map[string]interface{}{
						common.AnnotationKeyIgnoreDifferences: "/spec/replicas",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "test",
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "nginx",
									"image": "nginx:1.14.2",
								},
							},
						},
					},
				},
			},
		}

		live := desired.DeepCopy()
		// Change image (not covered by the annotation)
		containers, _, _ := unstructured.NestedSlice(live.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]interface{})
			container["image"] = "nginx:1.15.0"
			_ = unstructured.SetNestedSlice(live.Object, containers, "spec", "template", "spec", "containers")
		}

		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{}, true, normalizers.IgnoreNormalizerOpts{}).
			WithTracking("", "").
			WithNoCache().
			Build()
		require.NoError(t, err)

		// when
		result, err := argo.StateDiff(context.Background(), live, desired, diffConfig)

		// then
		require.NoError(t, err)
		assert.True(t, result.Modified, "Deployment should be modified because image change is not ignored")
	})

	t.Run("handles resource with empty group (core resources)", func(t *testing.T) {
		desired := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1", // No group!
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						common.AnnotationKeyIgnoreDifferences: "/spec/clusterIP",
					},
				},
			},
		}

		live := desired.DeepCopy()
		// Live state has cluster IP (should be ignored due to annotation)
		_ = unstructured.SetNestedField(live.Object, int64(5), "spec", "clusterIP")

		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{}, true, normalizers.IgnoreNormalizerOpts{}).
			WithTracking("", "").
			WithNoCache().
			Build()
		require.NoError(t, err)

		// when
		result, err := argo.StateDiff(context.Background(), live, desired, diffConfig)

		// then
		require.NoError(t, err)
		assert.False(t, result.Modified, "Service should not be modified because clusterIP is ignored via annotation")
	})

	t.Run("annotation with invalid JSON pointer - annotation is ignored", func(t *testing.T) {
		t.Parallel()
		desired := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-deployment",
					"namespace": "default",
					"annotations": map[string]interface{}{
						common.AnnotationKeyIgnoreDifferences: "spec/replicas",
					},
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "test",
						},
					},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "nginx",
									"image": "nginx:1.14.2",
								},
							},
						},
					},
				},
			},
		}

		live := desired.DeepCopy()
		// Live state has different replicas (should be ignored due to annotation)
		_ = unstructured.SetNestedField(live.Object, int64(5), "spec", "replicas")

		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{}, false, normalizers.IgnoreNormalizerOpts{}).
			WithTracking("", "").
			WithNoCache().
			Build()
		require.NoError(t, err)

		// when
		result, err := argo.StateDiff(context.Background(), live, desired, diffConfig)

		// then
		require.NoError(t, err)
		assert.True(t, result.Modified, "Deployment should be modified because annotation is invalid, thus ignored")
		normalized := testutil.YamlToUnstructured(string(result.NormalizedLive))
		replicas, found, err := unstructured.NestedFloat64(normalized.Object, "spec", "replicas")
		require.NoError(t, err)
		assert.True(t, found)
		assert.InEpsilon(t, float64(5), replicas, 0.0001)
	})
}
