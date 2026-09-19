package diff_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/common"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	testutil "github.com/argoproj/argo-cd/v3/test"
	argo "github.com/argoproj/argo-cd/v3/util/argo/diff"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/argo/testdata"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v3/util/cache/appstate"
)

func TestStateDiff(t *testing.T) {
	t.Parallel()
	type diffConfigParams struct {
		ignores        []v1alpha1.ResourceIgnoreDifferences
		overrides      map[string]v1alpha1.ResourceOverride
		label          string
		trackingMethod string
		ignoreRoles    bool
	}
	defaultDiffConfigParams := func() *diffConfigParams {
		return &diffConfigParams{
			ignores:        []v1alpha1.ResourceIgnoreDifferences{},
			overrides:      map[string]v1alpha1.ResourceOverride{},
			label:          "",
			trackingMethod: "",
			ignoreRoles:    true,
		}
	}
	diffConfig := func(t *testing.T, params *diffConfigParams) argo.DiffConfig {
		t.Helper()
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(params.ignores, params.overrides, params.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(params.label, params.trackingMethod).
			WithNoCache().
			Build()
		require.NoError(t, err)
		return diffConfig
	}
	type testcase struct {
		name                       string
		params                     func() *diffConfigParams
		desiredState               *unstructured.Unstructured
		liveState                  *unstructured.Unstructured
		expectedNormalizedReplicas int
		expectedPredictedReplicas  int
	}
	testcases := []*testcase{
		{
			name: "will normalize replica field if owned by trusted manager",
			params: func() *diffConfigParams {
				params := defaultDiffConfigParams()
				params.ignores = []v1alpha1.ResourceIgnoreDifferences{
					{
						Group:                 "*",
						Kind:                  "*",
						ManagedFieldsManagers: []string{"kube-controller-manager"},
					},
				}
				return params
			},
			desiredState:               testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml),
			liveState:                  testutil.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml),
			expectedNormalizedReplicas: 1,
			expectedPredictedReplicas:  1,
		},
		{
			name: "will keep replica field not owned by trusted manager",
			params: func() *diffConfigParams {
				params := defaultDiffConfigParams()
				params.ignores = []v1alpha1.ResourceIgnoreDifferences{
					{
						Group:                 "*",
						Kind:                  "*",
						ManagedFieldsManagers: []string{"some-other-manager"},
					},
				}
				return params
			},
			desiredState:               testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml),
			liveState:                  testutil.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml),
			expectedNormalizedReplicas: 2,
			expectedPredictedReplicas:  3,
		},
		{
			name: "will normalize replica field if configured with json pointers",
			params: func() *diffConfigParams {
				params := defaultDiffConfigParams()
				params.ignores = []v1alpha1.ResourceIgnoreDifferences{
					{
						Group:        "*",
						Kind:         "*",
						JSONPointers: []string{"/spec/replicas"},
					},
				}
				return params
			},
			desiredState:               testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml),
			liveState:                  testutil.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml),
			expectedNormalizedReplicas: 1,
			expectedPredictedReplicas:  1,
		},
		{
			name: "will normalize replica field if configured with jq expression",
			params: func() *diffConfigParams {
				params := defaultDiffConfigParams()
				params.ignores = []v1alpha1.ResourceIgnoreDifferences{
					{
						Group:             "*",
						Kind:              "*",
						JQPathExpressions: []string{".spec.replicas"},
					},
				}
				return params
			},
			desiredState:               testutil.YamlToUnstructured(testdata.DesiredDeploymentYaml),
			liveState:                  testutil.YamlToUnstructured(testdata.LiveDeploymentWithManagedReplicaYaml),
			expectedNormalizedReplicas: 1,
			expectedPredictedReplicas:  1,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// given
			t.Parallel()
			dc := diffConfig(t, tc.params())

			// when
			result, err := argo.StateDiff(t.Context(), tc.liveState, tc.desiredState, dc)

			// then
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.True(t, result.Modified)
			normalized := testutil.YamlToUnstructured(string(result.NormalizedLive))
			replicas, found, err := unstructured.NestedFloat64(normalized.Object, "spec", "replicas")
			require.NoError(t, err)
			assert.True(t, found)
			assert.InEpsilon(t, float64(tc.expectedNormalizedReplicas), replicas, 0.0001)
			predicted := testutil.YamlToUnstructured(string(result.PredictedLive))
			predictedReplicas, found, err := unstructured.NestedFloat64(predicted.Object, "spec", "replicas")
			require.NoError(t, err)
			assert.True(t, found)
			assert.InEpsilon(t, float64(tc.expectedPredictedReplicas), predictedReplicas, 0.0001)
		})
	}
}

func TestDiffConfigBuilder(t *testing.T) {
	t.Parallel()
	type fixture struct {
		ignores        []v1alpha1.ResourceIgnoreDifferences
		overrides      map[string]v1alpha1.ResourceOverride
		label          string
		trackingMethod string
		noCache        bool
		ignoreRoles    bool
		appName        string
	}
	setup := func() *fixture {
		return &fixture{
			ignores:        []v1alpha1.ResourceIgnoreDifferences{},
			overrides:      make(map[string]v1alpha1.ResourceOverride),
			label:          "some-label",
			trackingMethod: "tracking-method",
			noCache:        true,
			ignoreRoles:    false,
			appName:        "application-name",
		}
	}
	t.Run("will build diff config successfully", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()

		// when
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(f.ignores, f.overrides, f.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(f.label, f.trackingMethod).
			WithNoCache().
			Build()

		// then
		require.NoError(t, err)
		require.NotNil(t, diffConfig)
		assert.Empty(t, diffConfig.Ignores())
		assert.Empty(t, diffConfig.Overrides())
		assert.Equal(t, f.label, diffConfig.AppLabelKey())
		assert.Equal(t, f.overrides, diffConfig.Overrides())
		assert.Equal(t, f.trackingMethod, diffConfig.TrackingMethod())
		assert.Equal(t, f.noCache, diffConfig.NoCache())
		assert.Equal(t, f.ignoreRoles, diffConfig.IgnoreAggregatedRoles())
		assert.Empty(t, diffConfig.AppName())
		assert.Nil(t, diffConfig.StateCache())
	})
	t.Run("will initialize ignore differences if nil is passed", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()

		// when
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(nil, nil, f.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(f.label, f.trackingMethod).
			WithNoCache().
			Build()

		// then
		require.NoError(t, err)
		require.NotNil(t, diffConfig)
		assert.Empty(t, diffConfig.Ignores())
		assert.Empty(t, diffConfig.Overrides())
		assert.Equal(t, f.label, diffConfig.AppLabelKey())
		assert.Equal(t, f.overrides, diffConfig.Overrides())
		assert.Equal(t, f.trackingMethod, diffConfig.TrackingMethod())
		assert.Equal(t, f.noCache, diffConfig.NoCache())
		assert.Equal(t, f.ignoreRoles, diffConfig.IgnoreAggregatedRoles())
	})
	t.Run("will return error if retrieving diff from cache an no appName configured", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()

		// when
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(f.ignores, f.overrides, f.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(f.label, f.trackingMethod).
			WithCache(&appstatecache.Cache{}, "").
			Build()

		// then
		require.Error(t, err)
		require.Nil(t, diffConfig)
	})
	t.Run("will return error if retrieving diff from cache and no stateCache configured", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()

		// when
		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings(f.ignores, f.overrides, f.ignoreRoles, normalizers.IgnoreNormalizerOpts{}).
			WithTracking(f.label, f.trackingMethod).
			WithCache(nil, f.appName).
			Build()

		// then
		require.Error(t, err)
		require.Nil(t, diffConfig)
	})
}

func TestDiffFromCache(t *testing.T) {
	t.Run("returns false and logs warning on cache miss", func(t *testing.T) {
		// given
		hook := test.NewLocal(logrus.StandardLogger())
		defer hook.Reset()

		// Real in-memory cache with no data stored → triggers ErrCacheMiss
		cache := appstatecache.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(0)), 0)

		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{}, false, normalizers.IgnoreNormalizerOpts{}).
			WithTracking("", "").
			WithCache(cache, "application-name").
			Build()
		require.NoError(t, err)

		// when
		found, cachedDiff := diffConfig.DiffFromCache("application-name")

		// then
		assert.False(t, found)
		assert.Nil(t, cachedDiff)
		require.Len(t, hook.Entries, 1)
		assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
		assert.Contains(t, hook.LastEntry().Message, "cannot get managed resources for app application-name")
		assert.Contains(t, hook.LastEntry().Message, appstatecache.ErrCacheMiss.Error())
	})

	t.Run("returns false and logs error on cache failure", func(t *testing.T) {
		// given
		hook := test.NewLocal(logrus.StandardLogger())
		defer hook.Reset()

		errCache := errors.New("cache unavailable")
		// Custom cache client that always returns the given error on Get
		failClient := &failingCacheClient{
			InMemoryCache: cacheutil.NewInMemoryCache(0),
			err:           errCache,
		}
		cache := appstatecache.NewCache(cacheutil.NewCache(failClient), 0)

		diffConfig, err := argo.NewDiffConfigBuilder().
			WithDiffSettings([]v1alpha1.ResourceIgnoreDifferences{}, map[string]v1alpha1.ResourceOverride{}, false, normalizers.IgnoreNormalizerOpts{}).
			WithTracking("", "").
			WithCache(cache, "application-name").
			Build()
		require.NoError(t, err)

		// when
		found, cachedDiff := diffConfig.DiffFromCache("application-name")

		// then
		assert.False(t, found)
		assert.Nil(t, cachedDiff)
		require.Len(t, hook.Entries, 1)
		assert.Equal(t, logrus.ErrorLevel, hook.LastEntry().Level)
		assert.Contains(t, hook.LastEntry().Message, "cannot get managed resources for app application-name")
		assert.Contains(t, hook.LastEntry().Message, errCache.Error())
	})
}

// failingCacheClient embeds InMemoryCache and overrides Get to always return a custom error.
type failingCacheClient struct {
	*cacheutil.InMemoryCache
	err error
}

func (f *failingCacheClient) Get(_ string, _ any) error {
	return f.err
}

func TestStateDiffWithAnnotationBasedIgnoreDifferences(t *testing.T) {
	t.Parallel()

	t.Run("ignores differences based on resource annotation", func(t *testing.T) {
		// given
		t.Parallel()
		desired := testutil.YamlToUnstructured(testdata.DesiredDeploymentAnnotationYaml)

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
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "test-deployment",
					"namespace": "default",
					"annotations": map[string]any{
						common.AnnotationKeyIgnoreDifferences: "jsonPointers:\n- /spec/replicas\n- /metadata/labels/version",
					},
					"labels": map[string]any{
						"app":     "test",
						"version": "1.0",
					},
				},
				"spec": map[string]any{
					"replicas": int64(3),
					"selector": map[string]any{
						"matchLabels": map[string]any{
							"app": "test",
						},
					},
					"template": map[string]any{
						"metadata": map[string]any{
							"labels": map[string]any{
								"app": "test",
							},
						},
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
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
		desired := testutil.YamlToUnstructured(testdata.DesiredDeploymentAnnotationYaml)

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
		desired := testutil.YamlToUnstructured(testdata.DesiredDeploymentAnnotationYaml)

		live := desired.DeepCopy()
		// Change image (not covered by the annotation)
		containers, _, _ := unstructured.NestedSlice(live.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			container := containers[0].(map[string]any)
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
			Object: map[string]any{
				"apiVersion": "v1", // No group!
				"kind":       "Service",
				"metadata": map[string]any{
					"annotations": map[string]any{
						common.AnnotationKeyIgnoreDifferences: "jsonPointers:\n- /spec/clusterIP",
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
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "test-deployment",
					"namespace": "default",
					"annotations": map[string]any{
						common.AnnotationKeyIgnoreDifferences: "not a valid yaml struct for this annotation",
					},
				},
				"spec": map[string]any{
					"replicas": int64(3),
					"selector": map[string]any{
						"matchLabels": map[string]any{
							"app": "test",
						},
					},
					"template": map[string]any{
						"metadata": map[string]any{
							"labels": map[string]any{
								"app": "test",
							},
						},
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
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
