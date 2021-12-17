package argo

import (
	"fmt"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo/managedfields"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/go-logr/logr"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DiffConfig defines the configurations used while applying diffs.
type DiffConfig struct {
	// Ignores is list of ignore differences configuration set in the Application.
	Ignores []v1alpha1.ResourceIgnoreDifferences
	// SettingsMgr is used to retrieve the necessary system settings that are missing
	// in this DiffConfig
	SettingsMgr *settings.SettingsManager
	// Overrides is map of system configurations to override the Application ones.
	// The key should follow the "group/kind" format.
	Overrides      map[string]v1alpha1.ResourceOverride
	AppLabelKey    string
	TrackingMethod string
	// AppName the Application name. Used to retrieve the cached diff.
	AppName string
	// NoCache defines if should retrieve the diff from cache.
	NoCache bool
	// StateCache is used when retrieving the diff from the cache.
	StateCache            *appstatecache.Cache
	IgnoreAggregatedRoles bool
	// Logger used during the diff
	Logger logr.Logger
}

type normalizeResults struct {
	lives   []*unstructured.Unstructured
	targets []*unstructured.Unstructured
}

func StateDiff(live, config *unstructured.Unstructured, diffConfig *DiffConfig) (diff.DiffResult, error) {
	results, err := StateDiffs([]*unstructured.Unstructured{live}, []*unstructured.Unstructured{config}, diffConfig)
	if err != nil {
		return diff.DiffResult{}, err
	}
	if len(results.Diffs) != 1 {
		return diff.DiffResult{}, fmt.Errorf("StateDiff error: unexpected diff results: expected 1 got %d", len(results.Diffs))
	}
	return results.Diffs[0], nil
}

// TODO implement the validateDiffConfig
func validateDiffConfig(diffConfig *DiffConfig) error {
	// Overrides      map[string]v1alpha1.ResourceOverride
	// AppLabelKey    string
	// TrackingMethod string
	// NoCache bool
	// AppName string
	// StateCache            *appstatecache.Cache
	// IgnoreAggregatedRoles bool

	return nil
}

func StateDiffs(lives, configs []*unstructured.Unstructured, diffConfig *DiffConfig) (*diff.DiffResultList, error) {
	err := validateDiffConfig(diffConfig)

	normResults, err := preDiffNormalize(lives, configs, diffConfig)
	if err != nil {
		return nil, err
	}

	diffNormalizer, err := NewDiffNormalizer(diffConfig.Ignores, diffConfig.Overrides)
	if err != nil {
		return nil, err
	}
	diffOpts := []diff.Option{
		diff.WithNormalizer(diffNormalizer),
		diff.IgnoreAggregatedRoles(diffConfig.IgnoreAggregatedRoles),
	}

	if diffConfig.Logger != nil {
		diffOpts = append(diffOpts, diff.WithLogr(diffConfig.Logger))
	}

	useCache, cachedDiff := diffConfig.diffFromCache(diffConfig.AppName)
	if useCache && cachedDiff != nil {
		return diffArrayCached(normResults.targets, normResults.lives, cachedDiff, diffOpts...)
	}
	return diff.DiffArray(normResults.targets, normResults.lives, diffOpts...)
}

func diffArrayCached(configArray []*unstructured.Unstructured, liveArray []*unstructured.Unstructured, cachedDiff []*appv1.ResourceDiff, opts ...diff.Option) (*diff.DiffResultList, error) {
	numItems := len(configArray)
	if len(liveArray) != numItems {
		return nil, fmt.Errorf("left and right arrays have mismatched lengths")
	}

	diffByKey := map[kube.ResourceKey]*appv1.ResourceDiff{}
	for i := range cachedDiff {
		res := cachedDiff[i]
		diffByKey[kube.NewResourceKey(res.Group, res.Kind, res.Namespace, res.Name)] = cachedDiff[i]
	}

	diffResultList := diff.DiffResultList{
		Diffs: make([]diff.DiffResult, numItems),
	}

	for i := 0; i < numItems; i++ {
		config := configArray[i]
		live := liveArray[i]
		resourceVersion := ""
		var key kube.ResourceKey
		if live != nil {
			key = kube.GetResourceKey(live)
			resourceVersion = live.GetResourceVersion()
		} else {
			key = kube.GetResourceKey(config)
		}
		var dr *diff.DiffResult
		if cachedDiff, ok := diffByKey[key]; ok && cachedDiff.ResourceVersion == resourceVersion {
			dr = &diff.DiffResult{
				NormalizedLive: []byte(cachedDiff.NormalizedLiveState),
				PredictedLive:  []byte(cachedDiff.PredictedLiveState),
				Modified:       cachedDiff.Modified,
			}
		} else {
			res, err := diff.Diff(configArray[i], liveArray[i], opts...)
			if err != nil {
				return nil, err
			}
			dr = res
		}
		if dr != nil {
			diffResultList.Diffs[i] = *dr
			if dr.Modified {
				diffResultList.Modified = true
			}
		}
	}

	return &diffResultList, nil
}

// diffFromCache will verify if it should retrieve the cached ResourceDiff based on this
// DiffConfig. Returns true and the cached ResourceDiff if configured to use the cache.
// Returns false and nil otherwise.
func (c *DiffConfig) diffFromCache(appName string) (bool, []*appv1.ResourceDiff) {
	if c.NoCache || c.StateCache == nil || appName == "" {
		return false, nil
	}
	cachedDiff := make([]*appv1.ResourceDiff, 0)
	if c.StateCache != nil && c.StateCache.GetAppManagedResources(appName, &cachedDiff) != nil {
		return true, cachedDiff
	}
	return false, nil
}

// preDiffNormalize applies the normalization of live and target resources before invoking
// the diff. None of the attributes in the preDiffNormalizeParams will be modified. The
// normalizeResults will return a list of ApplicationConditions in case something goes
// wrong during the normalization.
func preDiffNormalize(lives, targets []*unstructured.Unstructured, diffConfig *DiffConfig) (*normalizeResults, error) {
	results := &normalizeResults{}
	for i := range targets {
		target := safeDeepCopy(targets[i])
		live := safeDeepCopy(lives[i])
		resourceTracking := NewResourceTracking()
		_ = resourceTracking.Normalize(target, live, diffConfig.AppLabelKey, diffConfig.TrackingMethod)
		// just normalize on managed fields if live and target aren't nil as we just care
		// about conflicting fields
		if live != nil && target != nil {
			gvk := target.GetObjectKind().GroupVersionKind()
			idc := NewIgnoreDiffConfig(diffConfig.Ignores, diffConfig.Overrides)
			ok, ignoreDiff := idc.HasIgnoreDifference(gvk.Group, gvk.Kind, target.GetName(), target.GetNamespace())
			if ok && len(ignoreDiff.ManagedFieldsManagers) > 0 {
				var err error
				live, target, err = managedfields.Normalize(live, target, ignoreDiff.ManagedFieldsManagers)
				if err != nil {
					return nil, err
				}
			}
		}
		results.lives = append(results.lives, live)
		results.targets = append(results.targets, target)
	}
	return results, nil
}

// safeDeepCopy will return nil if given obj is nil.
func safeDeepCopy(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}
	return obj.DeepCopy()
}
