package controller

import (
	"context"
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/kubectl/pkg/util/openapi"

	cdcommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/controller/metrics"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	listersv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/argo/diff"
	logutils "github.com/argoproj/argo-cd/v2/util/log"
	"github.com/argoproj/argo-cd/v2/util/lua"
	"github.com/argoproj/argo-cd/v2/util/rand"
)

var syncIdPrefix uint64 = 0

const (
	// EnvVarSyncWaveDelay is an environment variable which controls the delay in seconds between
	// each sync-wave
	EnvVarSyncWaveDelay = "ARGOCD_SYNC_WAVE_DELAY"
)

func (m *appStateManager) getOpenAPISchema(server string) (openapi.Resources, error) {
	cluster, err := m.liveStateCache.GetClusterCache(server)
	if err != nil {
		return nil, err
	}
	return cluster.GetOpenAPISchema(), nil
}

func (m *appStateManager) getGVKParser(server string) (*managedfields.GvkParser, error) {
	cluster, err := m.liveStateCache.GetClusterCache(server)
	if err != nil {
		return nil, err
	}
	return cluster.GetGVKParser(), nil
}

func (m *appStateManager) SyncAppState(app *v1alpha1.Application, state *v1alpha1.OperationState) {
	// Sync requests might be requested with ambiguous revisions (e.g. master, HEAD, v1.2.3).
	// This can change meaning when resuming operations (e.g a hook sync). After calculating a
	// concrete git commit SHA, the SHA is remembered in the status.operationState.syncResult field.
	// This ensures that when resuming an operation, we sync to the same revision that we initially
	// started with.
	var revision string
	var syncOp v1alpha1.SyncOperation
	var syncRes *v1alpha1.SyncOperationResult
	var source v1alpha1.ApplicationSource

	if state.Operation.Sync == nil {
		state.Phase = common.OperationFailed
		state.Message = "Invalid operation request: no operation specified"
		return
	}
	syncOp = *state.Operation.Sync

	// validates if it should fail the sync if it finds shared resources
	hasSharedResource, sharedResourceMessage := hasSharedResourceCondition(app)
	if syncOp.SyncOptions.HasOption("FailOnSharedResource=true") &&
		hasSharedResource {
		state.Phase = common.OperationFailed
		state.Message = fmt.Sprintf("Shared resouce found: %s", sharedResourceMessage)
		return
	}

	if syncOp.Source == nil {
		// normal sync case (where source is taken from app.spec.source)
		source = app.Spec.Source
	} else {
		// rollback case
		source = *state.Operation.Sync.Source
	}

	if state.SyncResult != nil {
		syncRes = state.SyncResult
		revision = state.SyncResult.Revision
	} else {
		syncRes = &v1alpha1.SyncOperationResult{}
		// status.operationState.syncResult.source. must be set properly since auto-sync relies
		// on this information to decide if it should sync (if source is different than the last
		// sync attempt)
		syncRes.Source = source
		state.SyncResult = syncRes
	}

	if revision == "" {
		// if we get here, it means we did not remember a commit SHA which we should be syncing to.
		// This typically indicates we are just about to begin a brand new sync/rollback operation.
		// Take the value in the requested operation. We will resolve this to a SHA later.
		revision = syncOp.Revision
	}

	proj, err := argo.GetAppProject(app, listersv1alpha1.NewAppProjectLister(m.projInformer.GetIndexer()), m.namespace, m.settingsMgr, m.db, context.TODO())
	if err != nil {
		state.Phase = common.OperationError
		state.Message = fmt.Sprintf("Failed to load application project: %v", err)
		return
	}

	compareResult := m.CompareAppState(app, proj, revision, source, false, true, syncOp.Manifests)
	// We now have a concrete commit SHA. Save this in the sync result revision so that we remember
	// what we should be syncing to when resuming operations.
	syncRes.Revision = compareResult.syncStatus.Revision

	// If there are any comparison or spec errors error conditions do not perform the operation
	if errConditions := app.Status.GetConditions(map[v1alpha1.ApplicationConditionType]bool{
		v1alpha1.ApplicationConditionComparisonError:  true,
		v1alpha1.ApplicationConditionInvalidSpecError: true,
	}); len(errConditions) > 0 {
		state.Phase = common.OperationError
		state.Message = argo.FormatAppConditions(errConditions)
		return
	}

	clst, err := m.db.GetCluster(context.Background(), app.Spec.Destination.Server)
	if err != nil {
		state.Phase = common.OperationError
		state.Message = err.Error()
		return
	}

	rawConfig := clst.RawRestConfig()
	restConfig := metrics.AddMetricsTransportWrapper(m.metricsServer, app, clst.RESTConfig())

	resourceOverrides, err := m.settingsMgr.GetResourceOverrides()
	if err != nil {
		state.Phase = common.OperationError
		state.Message = fmt.Sprintf("Failed to load resource overrides: %v", err)
		return
	}

	atomic.AddUint64(&syncIdPrefix, 1)
	randSuffix, err := rand.String(5)
	if err != nil {
		state.Phase = common.OperationError
		state.Message = fmt.Sprintf("Failed generate random sync ID: %v", err)
		return
	}
	syncId := fmt.Sprintf("%05d-%s", syncIdPrefix, randSuffix)

	logEntry := log.WithFields(log.Fields{"application": app.QualifiedName(), "syncId": syncId})
	initialResourcesRes := make([]common.ResourceSyncResult, 0)
	for i, res := range syncRes.Resources {
		key := kube.ResourceKey{Group: res.Group, Kind: res.Kind, Namespace: res.Namespace, Name: res.Name}
		initialResourcesRes = append(initialResourcesRes, common.ResourceSyncResult{
			ResourceKey: key,
			Message:     res.Message,
			Status:      res.Status,
			HookPhase:   res.HookPhase,
			HookType:    res.HookType,
			SyncPhase:   res.SyncPhase,
			Version:     res.Version,
			Order:       i + 1,
		})
	}

	prunePropagationPolicy := v1.DeletePropagationForeground
	switch {
	case syncOp.SyncOptions.HasOption("PrunePropagationPolicy=background"):
		prunePropagationPolicy = v1.DeletePropagationBackground
	case syncOp.SyncOptions.HasOption("PrunePropagationPolicy=foreground"):
		prunePropagationPolicy = v1.DeletePropagationForeground
	case syncOp.SyncOptions.HasOption("PrunePropagationPolicy=orphan"):
		prunePropagationPolicy = v1.DeletePropagationOrphan
	}

	openAPISchema, err := m.getOpenAPISchema(clst.Server)
	if err != nil {
		state.Phase = common.OperationError
		state.Message = fmt.Sprintf("failed to load openAPISchema: %v", err)
		return
	}

	reconciliationResult := compareResult.reconciliationResult

	// if RespectIgnoreDifferences is enabled, it should normalize the target
	// resources which in this case applies the live values in the configured
	// ignore differences fields.
	if syncOp.SyncOptions.HasOption("RespectIgnoreDifferences=true") {
		patchedTargets, err := normalizeTargetResources(compareResult)
		if err != nil {
			state.Phase = common.OperationError
			state.Message = fmt.Sprintf("Failed to normalize target resources: %s", err)
			return
		}
		reconciliationResult.Target = patchedTargets
	}

	appLabelKey, err := m.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		log.Errorf("Could not get appInstanceLabelKey: %v", err)
		return
	}
	trackingMethod := argo.GetTrackingMethod(m.settingsMgr)

	syncCtx, cleanup, err := sync.NewSyncContext(
		compareResult.syncStatus.Revision,
		reconciliationResult,
		restConfig,
		rawConfig,
		m.kubectl,
		app.Spec.Destination.Namespace,
		openAPISchema,
		sync.WithLogr(logutils.NewLogrusLogger(logEntry)),
		sync.WithHealthOverride(lua.ResourceHealthOverrides(resourceOverrides)),
		sync.WithPermissionValidator(func(un *unstructured.Unstructured, res *v1.APIResource) error {
			if !proj.IsGroupKindPermitted(un.GroupVersionKind().GroupKind(), res.Namespaced) {
				return fmt.Errorf("resource %s:%s is not permitted in project %s", un.GroupVersionKind().Group, un.GroupVersionKind().Kind, proj.Name)
			}
			if res.Namespaced && !proj.IsDestinationPermitted(v1alpha1.ApplicationDestination{Namespace: un.GetNamespace(), Server: app.Spec.Destination.Server, Name: app.Spec.Destination.Name}) {
				return fmt.Errorf("namespace %v is not permitted in project '%s'", un.GetNamespace(), proj.Name)
			}
			return nil
		}),
		sync.WithOperationSettings(syncOp.DryRun, syncOp.Prune, syncOp.SyncStrategy.Force(), syncOp.IsApplyStrategy() || len(syncOp.Resources) > 0),
		sync.WithInitialState(state.Phase, state.Message, initialResourcesRes, state.StartedAt),
		sync.WithResourcesFilter(func(key kube.ResourceKey, target *unstructured.Unstructured, live *unstructured.Unstructured) bool {
			return (len(syncOp.Resources) == 0 ||
				argo.ContainsSyncResource(key.Name, key.Namespace, schema.GroupVersionKind{Kind: key.Kind, Group: key.Group}, syncOp.Resources)) &&
				m.isSelfReferencedObj(live, appLabelKey, trackingMethod)
		}),
		sync.WithManifestValidation(!syncOp.SyncOptions.HasOption(common.SyncOptionsDisableValidation)),
		sync.WithNamespaceCreation(syncOp.SyncOptions.HasOption("CreateNamespace=true"), func(un *unstructured.Unstructured) bool {
			if un != nil && kube.GetAppInstanceLabel(un, cdcommon.LabelKeyAppInstance) != "" {
				kube.UnsetLabel(un, cdcommon.LabelKeyAppInstance)
				return true
			}
			return false
		}),
		sync.WithSyncWaveHook(delayBetweenSyncWaves),
		sync.WithPruneLast(syncOp.SyncOptions.HasOption(common.SyncOptionPruneLast)),
		sync.WithResourceModificationChecker(syncOp.SyncOptions.HasOption("ApplyOutOfSyncOnly=true"), compareResult.diffResultList),
		sync.WithPrunePropagationPolicy(&prunePropagationPolicy),
		sync.WithReplace(syncOp.SyncOptions.HasOption(common.SyncOptionReplace)),
		sync.WithServerSideApply(syncOp.SyncOptions.HasOption(common.SyncOptionServerSideApply)),
		sync.WithServerSideApplyManager(cdcommon.ArgoCDSSAManager),
	)

	if err != nil {
		state.Phase = common.OperationError
		state.Message = fmt.Sprintf("failed to initialize sync context: %v", err)
		return
	}

	defer cleanup()

	start := time.Now()

	if state.Phase == common.OperationTerminating {
		syncCtx.Terminate()
	} else {
		syncCtx.Sync()
	}
	var resState []common.ResourceSyncResult
	state.Phase, state.Message, resState = syncCtx.GetState()
	state.SyncResult.Resources = nil
	for _, res := range resState {
		state.SyncResult.Resources = append(state.SyncResult.Resources, &v1alpha1.ResourceResult{
			HookType:  res.HookType,
			Group:     res.ResourceKey.Group,
			Kind:      res.ResourceKey.Kind,
			Namespace: res.ResourceKey.Namespace,
			Name:      res.ResourceKey.Name,
			Version:   res.Version,
			SyncPhase: res.SyncPhase,
			HookPhase: res.HookPhase,
			Status:    res.Status,
			Message:   res.Message,
		})
	}

	logEntry.WithField("duration", time.Since(start)).Info("sync/terminate complete")

	if !syncOp.DryRun && len(syncOp.Resources) == 0 && state.Phase.Successful() {
		err := m.persistRevisionHistory(app, compareResult.syncStatus.Revision, source, state.StartedAt)
		if err != nil {
			state.Phase = common.OperationError
			state.Message = fmt.Sprintf("failed to record sync to history: %v", err)
		}
	}
}

func normalizeTargetResources(cr *comparisonResult) ([]*unstructured.Unstructured, error) {
	var patchedTargets []*unstructured.Unstructured

	diffConfig, err := diff.GetNormalizeConfig(cr.reconciliationResult.Live, cr.reconciliationResult.Target, cr.diffConfig)
	if err != nil {
		return nil, err
	}

	for idx, live := range cr.reconciliationResult.Live {
		target := cr.reconciliationResult.Target[idx]
		immutabilityConfig := diffConfig.Lives[idx]

		result := intersectMapWithImmutablePaths(live.Object, target.Object, immutabilityConfig, []interface{}{})

		patchedTargets = append(patchedTargets, &unstructured.Unstructured{Object: result})
	}

	return patchedTargets, nil
}

func anyMatch(x []interface{}, y [][]interface{}) bool {
	for _, path := range y {
		if reflect.DeepEqual(x, path) {
			return true
		}
	}
	return false
}

func intersectMapWithImmutablePaths(templateMap, valueMap map[string]interface{}, mapping diff.ImmutabilityMapping, currentPath []interface{}) map[string]interface{} {
	if anyMatch(currentPath, mapping.ImmutablePaths) {
		return templateMap
	}

	result := make(map[string]interface{})
	for k, v := range templateMap {
		if innerTMap, ok := v.(map[string]interface{}); ok {
			if innerVMap, ok := valueMap[k].(map[string]interface{}); ok {
				result[k] = intersectMapWithImmutablePaths(innerTMap, innerVMap, mapping, append(currentPath, k))
			}
		} else if innerTSlice, ok := v.([]interface{}); ok {
			if innerVSlice, ok := valueMap[k].([]interface{}); ok {
				var items []interface{}
				maxLen := int(math.Max(float64(len(innerTSlice)), float64(len(innerVSlice))))

				for idx := 0; idx < maxLen; idx++ {
					if len(innerTSlice) <= idx {
						items = append(items, innerVSlice[idx])
						continue
					}

					if len(innerVSlice) <= idx {
						items = append(items, innerTSlice[idx])
						continue
					}

					innerTSliceValue := innerTSlice[idx]
					innerVSliceValue := innerVSlice[idx]

					if anyMatch(append(currentPath, idx), mapping.ImmutablePaths) {
						items = append(items, innerTSliceValue)
					} else if idx < len(innerVSlice) {
						if tSliceValueMap, ok := innerTSliceValue.(map[string]interface{}); ok {
							if vSliceValueMap, ok := innerVSliceValue.(map[string]interface{}); ok {
								item := intersectMapWithImmutablePaths(tSliceValueMap, vSliceValueMap, mapping, append(currentPath, k, idx))
								items = append(items, item)
							}
						}
					}
				}
				if len(items) > 0 {
					result[k] = items
				}
			}
		} else if anyMatch(append(currentPath, k), mapping.ImmutablePaths) {
			if _, ok := templateMap[k]; ok {
				result[k] = templateMap[k]
			}
		} else {
			if _, ok := valueMap[k]; ok {
				result[k] = valueMap[k]
			}
		}
	}
	return result
}

// hasSharedResourceCondition will check if the Application has any resource that has already
// been synced by another Application. If the resource is found in another Application it returns
// true along with a human-readable message of which specific resource has this condition.
func hasSharedResourceCondition(app *v1alpha1.Application) (bool, string) {
	for _, condition := range app.Status.Conditions {
		if condition.Type == v1alpha1.ApplicationConditionSharedResourceWarning {
			return true, condition.Message
		}
	}
	return false, ""
}

// delayBetweenSyncWaves is a gitops-engine SyncWaveHook which introduces an artificial delay
// between each sync wave. We introduce an artificial delay in order give other controllers a
// _chance_ to react to the spec change that we just applied. This is important because without
// this, Argo CD will likely assess resource health too quickly (against the stale object), causing
// hooks to fire prematurely. See: https://github.com/argoproj/argo-cd/issues/4669.
// Note, this is not foolproof, since a proper fix would require the CRD record
// status.observedGeneration coupled with a health.lua that verifies
// status.observedGeneration == metadata.generation
func delayBetweenSyncWaves(phase common.SyncPhase, wave int, finalWave bool) error {
	if !finalWave {
		delaySec := 2
		if delaySecStr := os.Getenv(EnvVarSyncWaveDelay); delaySecStr != "" {
			if val, err := strconv.Atoi(delaySecStr); err == nil {
				delaySec = val
			}
		}
		duration := time.Duration(delaySec) * time.Second
		time.Sleep(duration)
	}
	return nil
}
