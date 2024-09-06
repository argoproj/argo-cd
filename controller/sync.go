package controller

import (
	"context"
	goerrors "errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	cdcommon "github.com/argoproj/argo-cd/v2/common"

	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/argo-cd/v2/controller/metrics"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	listersv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/argo/diff"
	"github.com/argoproj/argo-cd/v2/util/glob"
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

// getResourceOperations will return the kubectl implementation of the ResourceOperations
// interface that provides functionality to manage kubernetes resources. Returns a
// cleanup function that must be called to remove the generated kube config for this
// server.
func (m *appStateManager) getResourceOperations(server string) (kube.ResourceOperations, func(), error) {
	clusterCache, err := m.liveStateCache.GetClusterCache(server)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting cluster cache: %w", err)
	}

	cluster, err := m.db.GetCluster(context.Background(), server)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting cluster: %w", err)
	}
	ops, cleanup, err := m.kubectl.ManageResources(cluster.RawRestConfig(), clusterCache.GetOpenAPISchema())
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kubectl ResourceOperations: %w", err)
	}
	return ops, cleanup, nil
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
	var sources []v1alpha1.ApplicationSource
	revisions := make([]string, 0)

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
		state.Message = fmt.Sprintf("Shared resource found: %s", sharedResourceMessage)
		return
	}

	isMultiSourceRevision := app.Spec.HasMultipleSources()
	rollback := len(syncOp.Sources) > 0 || syncOp.Source != nil
	if rollback {
		// rollback case
		if len(state.Operation.Sync.Sources) > 0 {
			sources = state.Operation.Sync.Sources
			isMultiSourceRevision = true
		} else {
			source = *state.Operation.Sync.Source
			sources = make([]v1alpha1.ApplicationSource, 0)
			isMultiSourceRevision = false
		}
	} else {
		// normal sync case (where source is taken from app.spec.sources)
		if app.Spec.HasMultipleSources() {
			sources = app.Spec.Sources
		} else {
			// normal sync case (where source is taken from app.spec.source)
			source = app.Spec.GetSource()
			sources = make([]v1alpha1.ApplicationSource, 0)
		}
	}

	if state.SyncResult != nil {
		syncRes = state.SyncResult
		revision = state.SyncResult.Revision
		revisions = append(revisions, state.SyncResult.Revisions...)
	} else {
		syncRes = &v1alpha1.SyncOperationResult{}
		// status.operationState.syncResult.source. must be set properly since auto-sync relies
		// on this information to decide if it should sync (if source is different than the last
		// sync attempt)
		if isMultiSourceRevision {
			syncRes.Sources = sources
		} else {
			syncRes.Source = source
		}
		state.SyncResult = syncRes
	}

	// if we get here, it means we did not remember a commit SHA which we should be syncing to.
	// This typically indicates we are just about to begin a brand new sync/rollback operation.
	// Take the value in the requested operation. We will resolve this to a SHA later.
	if isMultiSourceRevision {
		if len(revisions) != len(sources) {
			revisions = syncOp.Revisions
		}
	} else {
		if revision == "" {
			revision = syncOp.Revision
		}
	}

	proj, err := argo.GetAppProject(app, listersv1alpha1.NewAppProjectLister(m.projInformer.GetIndexer()), m.namespace, m.settingsMgr, m.db, context.TODO())
	if err != nil {
		state.Phase = common.OperationError
		state.Message = fmt.Sprintf("Failed to load application project: %v", err)
		return
	} else if syncWindowPreventsSync(app, proj) {
		// If the operation is currently running, simply let the user know the sync is blocked by a current sync window
		if state.Phase == common.OperationRunning {
			state.Message = "Sync operation blocked by sync window"
		}
		return
	}

	if !isMultiSourceRevision {
		sources = []v1alpha1.ApplicationSource{source}
		revisions = []string{revision}
	}

	// ignore error if CompareStateRepoError, this shouldn't happen as noRevisionCache is true
	compareResult, err := m.CompareAppState(app, proj, revisions, sources, false, true, syncOp.Manifests, isMultiSourceRevision, rollback)
	if err != nil && !goerrors.Is(err, CompareStateRepoError) {
		state.Phase = common.OperationError
		state.Message = err.Error()
		return
	}
	// We now have a concrete commit SHA. Save this in the sync result revision so that we remember
	// what we should be syncing to when resuming operations.

	syncRes.Revision = compareResult.syncStatus.Revision
	syncRes.Revisions = compareResult.syncStatus.Revisions

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

	if m.settingsMgr.IsImpersonationEnabled() {
		serviceAccountToImpersonate, err := deriveServiceAccountName(proj, app)
		if err != nil {
			state.Phase = common.OperationError
			state.Message = fmt.Sprintf("failed to find a matching service account to impersonate: %v", err)
			return
		}
		logEntry = logEntry.WithFields(log.Fields{"impersonationEnabled": "true", "serviceAccount": serviceAccountToImpersonate})
		// set the impersonation headers.
		rawConfig.Impersonate = rest.ImpersonationConfig{
			UserName: serviceAccountToImpersonate,
		}
		restConfig.Impersonate = rest.ImpersonationConfig{
			UserName: serviceAccountToImpersonate,
		}
	}

	opts := []sync.SyncOpt{
		sync.WithLogr(logutils.NewLogrusLogger(logEntry)),
		sync.WithHealthOverride(lua.ResourceHealthOverrides(resourceOverrides)),
		sync.WithPermissionValidator(func(un *unstructured.Unstructured, res *v1.APIResource) error {
			if !proj.IsGroupKindPermitted(un.GroupVersionKind().GroupKind(), res.Namespaced) {
				return fmt.Errorf("resource %s:%s is not permitted in project %s", un.GroupVersionKind().Group, un.GroupVersionKind().Kind, proj.Name)
			}
			if res.Namespaced {
				permitted, err := proj.IsDestinationPermitted(v1alpha1.ApplicationDestination{Namespace: un.GetNamespace(), Server: app.Spec.Destination.Server, Name: app.Spec.Destination.Name}, func(project string) ([]*v1alpha1.Cluster, error) {
					return m.db.GetProjectClusters(context.TODO(), project)
				})
				if err != nil {
					return err
				}

				if !permitted {
					return fmt.Errorf("namespace %v is not permitted in project '%s'", un.GetNamespace(), proj.Name)
				}
			}
			return nil
		}),
		sync.WithOperationSettings(syncOp.DryRun, syncOp.Prune, syncOp.SyncStrategy.Force(), syncOp.IsApplyStrategy() || len(syncOp.Resources) > 0),
		sync.WithInitialState(state.Phase, state.Message, initialResourcesRes, state.StartedAt),
		sync.WithResourcesFilter(func(key kube.ResourceKey, target *unstructured.Unstructured, live *unstructured.Unstructured) bool {
			return (len(syncOp.Resources) == 0 ||
				isPostDeleteHook(target) ||
				argo.ContainsSyncResource(key.Name, key.Namespace, schema.GroupVersionKind{Kind: key.Kind, Group: key.Group}, syncOp.Resources)) &&
				m.isSelfReferencedObj(live, target, app.GetName(), appLabelKey, trackingMethod)
		}),
		sync.WithManifestValidation(!syncOp.SyncOptions.HasOption(common.SyncOptionsDisableValidation)),
		sync.WithSyncWaveHook(delayBetweenSyncWaves),
		sync.WithPruneLast(syncOp.SyncOptions.HasOption(common.SyncOptionPruneLast)),
		sync.WithResourceModificationChecker(syncOp.SyncOptions.HasOption("ApplyOutOfSyncOnly=true"), compareResult.diffResultList),
		sync.WithPrunePropagationPolicy(&prunePropagationPolicy),
		sync.WithReplace(syncOp.SyncOptions.HasOption(common.SyncOptionReplace)),
		sync.WithServerSideApply(syncOp.SyncOptions.HasOption(common.SyncOptionServerSideApply)),
		sync.WithServerSideApplyManager(cdcommon.ArgoCDSSAManager),
	}

	if syncOp.SyncOptions.HasOption("CreateNamespace=true") {
		opts = append(opts, sync.WithNamespaceModifier(syncNamespace(app.Spec.SyncPolicy)))
	}

	syncCtx, cleanup, err := sync.NewSyncContext(
		compareResult.syncStatus.Revision,
		reconciliationResult,
		restConfig,
		rawConfig,
		m.kubectl,
		app.Spec.Destination.Namespace,
		openAPISchema,
		opts...,
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

	if app.Spec.SyncPolicy != nil {
		state.SyncResult.ManagedNamespaceMetadata = app.Spec.SyncPolicy.ManagedNamespaceMetadata
	}

	var apiVersion []kube.APIResourceInfo
	for _, res := range resState {
		augmentedMsg, err := argo.AugmentSyncMsg(res, func() ([]kube.APIResourceInfo, error) {
			if apiVersion == nil {
				_, apiVersion, err = m.liveStateCache.GetVersionsInfo(app.Spec.Destination.Server)
				if err != nil {
					return nil, fmt.Errorf("failed to get version info from the target cluster %q", app.Spec.Destination.Server)
				}
			}
			return apiVersion, nil
		})

		if err != nil {
			log.Errorf("using the original message since: %v", err)
		} else {
			res.Message = augmentedMsg
		}

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
		err := m.persistRevisionHistory(app, compareResult.syncStatus.Revision, source, compareResult.syncStatus.Revisions, compareResult.syncStatus.ComparedTo.Sources, isMultiSourceRevision, state.StartedAt, state.Operation.InitiatedBy)
		if err != nil {
			state.Phase = common.OperationError
			state.Message = fmt.Sprintf("failed to record sync to history: %v", err)
		}
	}
}

// normalizeTargetResources modifies target resources to ensure ignored fields are not touched during synchronization:
//   - applies normalization to the target resources based on the live resources
//   - copies ignored fields from the matching live resources: apply normalizer to the live resource,
//     calculates the patch performed by normalizer and applies the patch to the target resource
func normalizeTargetResources(cr *comparisonResult) ([]*unstructured.Unstructured, error) {
	// normalize live and target resources
	normalized, err := diff.Normalize(cr.reconciliationResult.Live, cr.reconciliationResult.Target, cr.diffConfig)
	if err != nil {
		return nil, err
	}
	patchedTargets := []*unstructured.Unstructured{}
	for idx, live := range cr.reconciliationResult.Live {
		normalizedTarget := normalized.Targets[idx]
		if normalizedTarget == nil {
			patchedTargets = append(patchedTargets, nil)
			continue
		}
		originalTarget := cr.reconciliationResult.Target[idx]
		if live == nil {
			patchedTargets = append(patchedTargets, originalTarget)
			continue
		}

		var lookupPatchMeta *strategicpatch.PatchMetaFromStruct
		versionedObject, err := scheme.Scheme.New(normalizedTarget.GroupVersionKind())
		if err == nil {
			meta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
			if err != nil {
				return nil, err
			}
			lookupPatchMeta = &meta
		}

		livePatch, err := getMergePatch(normalized.Lives[idx], live, lookupPatchMeta)
		if err != nil {
			return nil, err
		}

		normalizedTarget, err = applyMergePatch(normalizedTarget, livePatch, versionedObject)
		if err != nil {
			return nil, err
		}

		patchedTargets = append(patchedTargets, normalizedTarget)
	}
	return patchedTargets, nil
}

// getMergePatch calculates and returns the patch between the original and the
// modified unstructures.
func getMergePatch(original, modified *unstructured.Unstructured, lookupPatchMeta *strategicpatch.PatchMetaFromStruct) ([]byte, error) {
	originalJSON, err := original.MarshalJSON()
	if err != nil {
		return nil, err
	}
	modifiedJSON, err := modified.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if lookupPatchMeta != nil {
		return strategicpatch.CreateThreeWayMergePatch(modifiedJSON, modifiedJSON, originalJSON, lookupPatchMeta, true)
	}

	return jsonpatch.CreateMergePatch(originalJSON, modifiedJSON)
}

// applyMergePatch will apply the given patch in the obj and return the patched
// unstructure.
func applyMergePatch(obj *unstructured.Unstructured, patch []byte, versionedObject interface{}) (*unstructured.Unstructured, error) {
	originalJSON, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var patchedJSON []byte
	if versionedObject == nil {
		patchedJSON, err = jsonpatch.MergePatch(originalJSON, patch)
	} else {
		patchedJSON, err = strategicpatch.StrategicMergePatch(originalJSON, patch, versionedObject)
	}
	if err != nil {
		return nil, err
	}

	patchedObj := &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(patchedJSON, nil, patchedObj)
	if err != nil {
		return nil, err
	}
	return patchedObj, nil
}

// hasSharedResourceCondition will check if the Application has any resource that has already
// been synced by another Application. If the resource is found in another Application it returns
// true along with a human readable message of which specific resource has this condition.
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

func syncWindowPreventsSync(app *v1alpha1.Application, proj *v1alpha1.AppProject) bool {
	window := proj.Spec.SyncWindows.Matches(app)
	isManual := false
	if app.Status.OperationState != nil {
		isManual = !app.Status.OperationState.Operation.InitiatedBy.Automated
	}
	return !window.CanSync(isManual)
}

// deriveServiceAccountName determines the service account to be used for impersonation for the sync operation.
// The returned service account will be fully qualified including namespace and the service account name in the format system:serviceaccount:<namespace>:<service_account>
func deriveServiceAccountName(project *v1alpha1.AppProject, application *v1alpha1.Application) (string, error) {
	// spec.Destination.Namespace is optional. If not specified, use the Application's
	// namespace
	serviceAccountNamespace := application.Spec.Destination.Namespace
	if serviceAccountNamespace == "" {
		serviceAccountNamespace = application.Namespace
	}
	// Loop through the destinationServiceAccounts and see if there is any destination that is a candidate.
	// if so, return the service account specified for that destination.
	for _, item := range project.Spec.DestinationServiceAccounts {
		dstServerMatched := glob.Match(item.Server, application.Spec.Destination.Server)
		dstNamespaceMatched := glob.Match(item.Namespace, application.Spec.Destination.Namespace)
		if dstServerMatched && dstNamespaceMatched {
			if strings.Contains(item.DefaultServiceAccount, ":") {
				// service account is specified along with its namespace.
				return fmt.Sprintf("system:serviceaccount:%s", item.DefaultServiceAccount), nil
			} else {
				// service account needs to be prefixed with a namespace
				return fmt.Sprintf("system:serviceaccount:%s:%s", serviceAccountNamespace, item.DefaultServiceAccount), nil
			}
		}
	}
	// if there is no match found in the AppProject.Spec.DestinationServiceAccounts, use the default service account of the destination namespace.
	return "", fmt.Errorf("no matching service account found for destination server %s and namespace %s", application.Spec.Destination.Server, serviceAccountNamespace)
}
