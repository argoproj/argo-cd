package diff

import (
	"fmt"

	"github.com/go-logr/logr"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/argo/managedfields"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DiffConfigBuilder is used as a safe way to create valid DiffConfigs.
type DiffConfigBuilder struct {
	diffConfig *diffConfig
}

// NewDiffConfigBuilder create a new DiffConfigBuilder instance.
func NewDiffConfigBuilder() *DiffConfigBuilder {
	return &DiffConfigBuilder{
		diffConfig: &diffConfig{},
	}
}

// WithDiffSettings will set the diff settings in the builder.
func (b *DiffConfigBuilder) WithDiffSettings(id []v1alpha1.ResourceIgnoreDifferences, o map[string]v1alpha1.ResourceOverride, ignoreAggregatedRoles bool) *DiffConfigBuilder {
	ignores := id
	if ignores == nil {
		ignores = []v1alpha1.ResourceIgnoreDifferences{}
	}
	b.diffConfig.ignores = ignores

	overrides := o
	if overrides == nil {
		overrides = make(map[string]appv1.ResourceOverride)
	}
	b.diffConfig.overrides = overrides
	b.diffConfig.ignoreAggregatedRoles = ignoreAggregatedRoles
	return b
}

// WithTrackingMethod sets the tracking in the diff config.
func (b *DiffConfigBuilder) WithTracking(appLabelKey, trackingMethod string) *DiffConfigBuilder {
	b.diffConfig.appLabelKey = appLabelKey
	b.diffConfig.trackingMethod = trackingMethod
	return b
}

// WithNoCache sets the nocache in the diff config.
func (b *DiffConfigBuilder) WithNoCache() *DiffConfigBuilder {
	b.diffConfig.noCache = true
	return b
}

// WithCache sets the appstatecache.Cache and the appName in the diff config. Those the
// are two objects necessary to retrieve a cached diff.
func (b *DiffConfigBuilder) WithCache(s *appstatecache.Cache, appName string) *DiffConfigBuilder {
	b.diffConfig.noCache = false
	b.diffConfig.stateCache = s
	b.diffConfig.appName = appName
	return b
}

// WithLogger sets the logger in the diff config.
func (b *DiffConfigBuilder) WithLogger(l logr.Logger) *DiffConfigBuilder {
	b.diffConfig.logger = &l
	return b
}

// Build will first validate the current state of the diff config and return the
// DiffConfig implementation if no errors are found. Will return nil and the error
// details otherwise.
func (b *DiffConfigBuilder) Build() (DiffConfig, error) {
	err := b.diffConfig.Validate()
	if err != nil {
		return nil, err
	}
	return b.diffConfig, nil
}

// DiffConfig defines methods to retrieve the configurations used while applying diffs
// and normalizing resources.
type DiffConfig interface {
	// Validate will check if the current configurations are set properly.
	Validate() error
	// DiffFromCache will verify if it should retrieve the cached ResourceDiff based on this
	// DiffConfig.
	DiffFromCache(appName string) (bool, []*appv1.ResourceDiff)
	// Ignores Application level ignore difference configurations.
	Ignores() []v1alpha1.ResourceIgnoreDifferences
	// Overrides is map of system configurations to override the Application ones.
	// The key should follow the "group/kind" format.
	Overrides() map[string]v1alpha1.ResourceOverride
	AppLabelKey() string
	TrackingMethod() string
	// AppName the Application name. Used to retrieve the cached diff.
	AppName() string
	// NoCache defines if should retrieve the diff from cache.
	NoCache() bool
	// StateCache is used when retrieving the diff from the cache.
	StateCache() *appstatecache.Cache
	IgnoreAggregatedRoles() bool
	// Logger used during the diff
	Logger() *logr.Logger
}

// diffConfig defines the configurations used while applying diffs.
type diffConfig struct {
	ignores               []v1alpha1.ResourceIgnoreDifferences
	overrides             map[string]v1alpha1.ResourceOverride
	appLabelKey           string
	trackingMethod        string
	appName               string
	noCache               bool
	stateCache            *appstatecache.Cache
	ignoreAggregatedRoles bool
	logger                *logr.Logger
}

func (c *diffConfig) Ignores() []v1alpha1.ResourceIgnoreDifferences {
	return c.ignores
}
func (c *diffConfig) Overrides() map[string]v1alpha1.ResourceOverride {
	return c.overrides
}
func (c *diffConfig) AppLabelKey() string {
	return c.appLabelKey
}
func (c *diffConfig) TrackingMethod() string {
	return c.trackingMethod
}
func (c *diffConfig) AppName() string {
	return c.appName
}
func (c *diffConfig) NoCache() bool {
	return c.noCache
}
func (c *diffConfig) StateCache() *appstatecache.Cache {
	return c.stateCache
}
func (c *diffConfig) IgnoreAggregatedRoles() bool {
	return c.ignoreAggregatedRoles
}
func (c *diffConfig) Logger() *logr.Logger {
	return c.logger
}

// Validate will check the current state of this diffConfig and return
// error if it finds any required configuration missing.
func (c *diffConfig) Validate() error {
	msg := "diffConfig validation error"
	if c.ignores == nil {
		return fmt.Errorf("%s: ResourceIgnoreDifferences can not be nil", msg)
	}
	if c.overrides == nil {
		return fmt.Errorf("%s: ResourceOverride can not be nil", msg)
	}
	if !c.noCache {
		if c.appName == "" {
			return fmt.Errorf("%s: AppName must be set when retrieving from cache", msg)
		}
		if c.stateCache == nil {
			return fmt.Errorf("%s: StateCache must be set when retrieving from cache", msg)
		}
	}
	return nil
}

// NormalizationResult holds the normalized lives and target resources.
type NormalizationResult struct {
	Lives   []*unstructured.Unstructured
	Targets []*unstructured.Unstructured
}

// StateDiff will apply all required normalizations and calculate the diffs between
// the live and the config/desired states.
func StateDiff(live, config *unstructured.Unstructured, diffConfig DiffConfig) (diff.DiffResult, error) {
	results, err := StateDiffs([]*unstructured.Unstructured{live}, []*unstructured.Unstructured{config}, diffConfig)
	if err != nil {
		return diff.DiffResult{}, err
	}
	if len(results.Diffs) != 1 {
		return diff.DiffResult{}, fmt.Errorf("StateDiff error: unexpected diff results: expected 1 got %d", len(results.Diffs))
	}
	return results.Diffs[0], nil
}

// StateDiffs will apply all required normalizations and calculate the diffs between
// the live and the config/desired states.
func StateDiffs(lives, configs []*unstructured.Unstructured, diffConfig DiffConfig) (*diff.DiffResultList, error) {
	normResults, err := preDiffNormalize(lives, configs, diffConfig)
	if err != nil {
		return nil, err
	}

	diffNormalizer, err := newDiffNormalizer(diffConfig.Ignores(), diffConfig.Overrides())
	if err != nil {
		return nil, err
	}
	diffOpts := []diff.Option{
		diff.WithNormalizer(diffNormalizer),
		diff.IgnoreAggregatedRoles(diffConfig.IgnoreAggregatedRoles()),
	}

	if diffConfig.Logger() != nil {
		diffOpts = append(diffOpts, diff.WithLogr(*diffConfig.Logger()))
	}

	useCache, cachedDiff := diffConfig.DiffFromCache(diffConfig.AppName())
	if useCache && cachedDiff != nil {
		return diffArrayCached(normResults.Targets, normResults.Lives, cachedDiff, diffOpts...)
	}
	return diff.DiffArray(normResults.Targets, normResults.Lives, diffOpts...)
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

// DiffFromCache will verify if it should retrieve the cached ResourceDiff based on this
// DiffConfig. Returns true and the cached ResourceDiff if configured to use the cache.
// Returns false and nil otherwise.
func (c *diffConfig) DiffFromCache(appName string) (bool, []*appv1.ResourceDiff) {
	if c.noCache || c.stateCache == nil || appName == "" {
		return false, nil
	}
	cachedDiff := make([]*appv1.ResourceDiff, 0)
	if c.stateCache != nil && c.stateCache.GetAppManagedResources(appName, &cachedDiff) == nil {
		return true, cachedDiff
	}
	return false, nil
}

// preDiffNormalize applies the normalization of live and target resources before invoking
// the diff. None of the attributes in the preDiffNormalizeParams will be modified.
func preDiffNormalize(lives, targets []*unstructured.Unstructured, diffConfig DiffConfig) (*NormalizationResult, error) {
	if diffConfig == nil {
		return nil, fmt.Errorf("preDiffNormalize error: diffConfig can not be nil")
	}
	err := diffConfig.Validate()
	if err != nil {
		return nil, fmt.Errorf("preDiffNormalize error: %s", err)
	}

	results := &NormalizationResult{}
	for i := range targets {
		target := safeDeepCopy(targets[i])
		live := safeDeepCopy(lives[i])
		resourceTracking := argo.NewResourceTracking()
		_ = resourceTracking.Normalize(target, live, diffConfig.AppLabelKey(), diffConfig.TrackingMethod())
		// just normalize on managed fields if live and target aren't nil as we just care
		// about conflicting fields
		if live != nil && target != nil {
			gvk := target.GetObjectKind().GroupVersionKind()
			idc := NewIgnoreDiffConfig(diffConfig.Ignores(), diffConfig.Overrides())
			ok, ignoreDiff := idc.HasIgnoreDifference(gvk.Group, gvk.Kind, target.GetName(), target.GetNamespace())
			if ok && len(ignoreDiff.ManagedFieldsManagers) > 0 {
				var err error
				live, target, err = managedfields.Normalize(live, target, ignoreDiff.ManagedFieldsManagers)
				if err != nil {
					return nil, err
				}
			}
		}
		results.Lives = append(results.Lives, live)
		results.Targets = append(results.Targets, target)
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
