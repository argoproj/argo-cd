package diff_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	testutil "github.com/argoproj/argo-cd/v2/test"
	argo "github.com/argoproj/argo-cd/v2/util/argo/diff"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v2/util/argo/testdata"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
)

func TestStateDiff(t *testing.T) {
	type diffConfigParams struct {
		ignores        []v1alpha1.ResourceIgnoreDifferences
		overrides      map[string]v1alpha1.ResourceOverride
		label          string
		trackingMethod string
		noCache        bool
		ignoreRoles    bool
		appName        string
		stateCache     *appstatecache.Cache
	}
	defaultDiffConfigParams := func() *diffConfigParams {
		return &diffConfigParams{
			ignores:        []v1alpha1.ResourceIgnoreDifferences{},
			overrides:      map[string]v1alpha1.ResourceOverride{},
			label:          "",
			trackingMethod: "",
			noCache:        true,
			ignoreRoles:    true,
			appName:        "",
			stateCache:     &appstatecache.Cache{},
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// given
			dc := diffConfig(t, tc.params())

			// when
			result, err := argo.StateDiff(tc.liveState, tc.desiredState, dc)

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
	type fixture struct {
		ignores        []v1alpha1.ResourceIgnoreDifferences
		overrides      map[string]v1alpha1.ResourceOverride
		label          string
		trackingMethod string
		noCache        bool
		ignoreRoles    bool
		appName        string
		stateCache     *appstatecache.Cache
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
			stateCache:     &appstatecache.Cache{},
		}
	}
	t.Run("will build diff config successfully", func(t *testing.T) {
		// given
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
		assert.Equal(t, "", diffConfig.AppName())
		assert.Nil(t, diffConfig.StateCache())
	})
	t.Run("will initialize ignore differences if nil is passed", func(t *testing.T) {
		// given
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
