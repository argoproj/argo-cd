package controller

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	cdcommon "github.com/argoproj/argo-cd/v3/common"

	gitopsDiff "github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/argo-cd/v3/controller/metrics"
	"github.com/argoproj/argo-cd/v3/controller/syncid"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/argo/diff"
	kubeutil "github.com/argoproj/argo-cd/v3/util/kube"
	logutils "github.com/argoproj/argo-cd/v3/util/log"
	"github.com/argoproj/argo-cd/v3/util/lua"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	// EnvVarSyncWaveDelay is an environment variable which controls the delay in seconds between
	// each sync-wave
	EnvVarSyncWaveDelay = "ARGOCD_SYNC_WAVE_DELAY"
)

func (m *appStateManager) getOpenAPISchema(server *v1alpha1.Cluster) (openapi.Resources, error) {
	cluster, err := m.liveStateCache.GetClusterCache(server)
	if err != nil {
		return nil, err
	}
	return cluster.GetOpenAPISchema(), nil
}

func (m *appStateManager) getGVKParser(server *v1alpha1.Cluster) (*managedfields.GvkParser, error) {
	cluster, err := m.liveStateCache.GetClusterCache(server)
	if err != nil {
		return nil, err
	}
	return cluster.GetGVKParser(), nil
}

// getServerSideDiffDryRunApplier will return the kubectl implementation of the KubeApplier
// interface that provides functionality to dry run apply kubernetes resources. Returns a
// cleanup function that must be called to remove the generated kube config for this
// server.
func (m *appStateManager) getServerSideDiffDryRunApplier(cluster *v1alpha1.Cluster) (gitopsDiff.KubeApplier, func(), error) {
	rawConfig, err := cluster.RawRestConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting cluster REST config: %w", err)
	}
	ops, cleanup, err := kubeutil.ManageServerSideDiffDryRuns(rawConfig, m.onKubectlRun)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kubectl ResourceOperations: %w", err)
	}
	return ops, cleanup, nil
}

func NewOperationState(operation v1alpha1.Operation) *v1alpha1.OperationState {
	return &v1alpha1.OperationState{
		Phase:     common.OperationRunning,
		Operation: operation,
		StartedAt: metav1.Now(),
	}
}

func newSyncOperationResult(app *v1alpha1.Application, op v1alpha1.SyncOperation) *v1alpha1.SyncOperationResult {
	syncRes := &v1alpha1.SyncOperationResult{}

	if len(op.Sources) > 0 || op.Source != nil {
		// specific source specified in the SyncOperation
		if op.Source != nil {
			syncRes.Source = *op.Source
		}
		syncRes.Sources = op.Sources
	} else {
		// normal sync case, get sources from the spec
		syncRes.Sources = app.Spec.Sources
		syncRes.Source = app.Spec.GetSource()
	}

	// Sync requests might be requested with ambiguous revisions (e.g. master, HEAD, v1.2.3).
	// This can change meaning when resuming operations (e.g a hook sync). After calculating a
	// concrete git commit SHA, the revision of the SyncOperationResult will be updated with the SHA
	syncRes.Revision = op.Revision
	syncRes.Revisions = op.Revisions
	return syncRes
}

func (m *appStateManager) SyncAppState(app *v1alpha1.Application, project *v1alpha1.AppProject, state *v1alpha1.OperationState) {
	syncId, err := syncid.Generate()
	if err != nil {
		state.Phase = common.OperationError
		state.Message = fmt.Sprintf("Failed to generate sync ID: %v", err)
		return
	}
	logEntry := log.WithFields(applog.GetAppLogFields(app)).WithField("syncId", syncId)

	if state.Operation.Sync == nil {
		state.Phase = common.OperationError
		state.Message = "Invalid operation request: no operation specified"
		return
	}

	syncOp := *state.Operation.Sync

	if state.SyncResult == nil {
		state.SyncResult = newSyncOperationResult(app, syncOp)
	}

	if isBlocked, err := syncWindowPreventsSync(app, project); isBlocked {
		// If the operation is currently running, simply let the user know the sync is blocked by a current sync window
		if state.Phase == common.OperationRunning {
			state.Message = "Sync operation blocked by sync window"
			if err != nil {
				state.Message = fmt.Sprintf("%s: %v", state.Message, err)
			}
		}
		return
	}

	revisions := state.SyncResult.Revisions
	sources := state.SyncResult.Sources
	isMultiSourceSync := len(sources) > 0
	if !isMultiSourceSync {
		sources = []v1alpha1.ApplicationSource{state.SyncResult.Source}
		revisions = []string{state.SyncResult.Revision}
	}

	// ignore error if CompareStateRepoError, this shouldn't happen as noRevisionCache is true
	compareResult, err := m.CompareAppState(app, project, revisions, sources, false, true, syncOp.Manifests, isMultiSourceSync)
	if err != nil && !stderrors.Is(err, ErrCompareStateRepo) {
		state.Phase = common.OperationError
		state.Message = err.Error()
		return
	}

	// We are now guaranteed to have a concrete commit SHA. Save this in the sync result revision so that we remember
	// what we should be syncing to when resuming operations.
	state.SyncResult.Revision = compareResult.syncStatus.Revision
	state.SyncResult.Revisions = compareResult.syncStatus.Revisions

	// validates if it should fail the sync on that revision if it finds shared resources
	hasSharedResource, sharedResourceMessage := hasSharedResourceCondition(app)
	if syncOp.SyncOptions.HasOption("FailOnSharedResource=true") && hasSharedResource {
		state.Phase = common.OperationFailed
		state.Message = "Shared resource found: " + sharedResourceMessage
		return
	}

	// If there are any comparison or spec errors error conditions do not perform the operation
	if errConditions := app.Status.GetConditions(map[v1alpha1.ApplicationConditionType]bool{
		v1alpha1.ApplicationConditionComparisonError:  true,
		v1alpha1.ApplicationConditionInvalidSpecError: true,
	}); len(errConditions) > 0 {
		state.Phase = common.OperationError
		state.Message = argo.FormatAppConditions(errConditions)
		return
	}

	destCluster, err := argo.GetDestinationCluster(context.Background(), app.Spec.Destination, m.db)
	if err != nil {
		state.Phase = common.OperationError
		state.Message = fmt.Sprintf("Failed to get destination cluster: %v", err)
		return
	}

	rawConfig, err := destCluster.RawRestConfig()
	if err != nil {
		state.Phase = common.OperationError
		state.Message = err.Error()
		return
	}

	clusterRESTConfig, err := destCluster.RESTConfig()
	if err != nil {
		state.Phase = common.OperationError
		state.Message = err.Error()
		return
	}
	restConfig := metrics.AddMetricsTransportWrapper(m.metricsServer, app, clusterRESTConfig)

	resourceOverrides, err := m.settingsMgr.GetResourceOverrides()
	if err != nil {
		state.Phase = common.OperationError
		state.Message = fmt.Sprintf("Failed to load resource overrides: %v", err)
		return
	}

	initialResourcesRes := make([]common.ResourceSyncResult, len(state.SyncResult.Resources))
	for i, res := range state.SyncResult.Resources {
		key := kube.ResourceKey{Group: res.Group, Kind: res.Kind, Namespace: res.Namespace, Name: res.Name}
		initialResourcesRes[i] = common.ResourceSyncResult{
			ResourceKey: key,
			Message:     res.Message,
			Status:      res.Status,
			HookPhase:   res.HookPhase,
			HookType:    res.HookType,
			SyncPhase:   res.SyncPhase,
			Version:     res.Version,
			Images:      res.Images,
			Order:       i + 1,
		}
	}

	prunePropagationPolicy := metav1.DeletePropagationForeground
	switch {
	case syncOp.SyncOptions.HasOption("PrunePropagationPolicy=background"):
		prunePropagationPolicy = metav1.DeletePropagationBackground
	case syncOp.SyncOptions.HasOption("PrunePropagationPolicy=foreground"):
		prunePropagationPolicy = metav1.DeletePropagationForeground
	case syncOp.SyncOptions.HasOption("PrunePropagationPolicy=orphan"):
		prunePropagationPolicy = metav1.DeletePropagationOrphan
	}

	clientSideApplyManager := common.DefaultClientSideApplyMigrationManager
	// Check for custom field manager from application annotation
	if managerValue := app.GetAnnotation(cdcommon.AnnotationClientSideApplyMigrationManager); managerValue != "" {
		clientSideApplyManager = managerValue
	}

	reconciliationResult := compareResult.reconciliationResult

	// if RespectIgnoreDifferences is enabled, it should normalize the target
	// resources which in this case applies the live values in the configured
	// ignore differences fields.
	if syncOp.SyncOptions.HasOption("RespectIgnoreDifferences=true") {
		openAPISchema, err := m.getOpenAPISchema(destCluster)
		if err != nil {
			state.Phase = common.OperationError
			state.Message = fmt.Sprintf("failed to load openAPISchema: %v", err)
			return
		}

		patchedTargets, err := normalizeTargetResources(openAPISchema, compareResult)
		if err != nil {
			state.Phase = common.OperationError
			state.Message = fmt.Sprintf("Failed to normalize target resources: %s", err)
			return
		}
		reconciliationResult.Target = patchedTargets
	}

	installationID, err := m.settingsMgr.GetInstallationID()
	if err != nil {
		log.Errorf("Could not get installation ID: %v", err)
		return
	}
	trackingMethod, err := m.settingsMgr.GetTrackingMethod()
	if err != nil {
		log.Errorf("Could not get trackingMethod: %v", err)
		return
	}

	impersonationEnabled, err := m.settingsMgr.IsImpersonationEnabled()
	if err != nil {
		log.Errorf("could not get impersonation feature flag: %v", err)
		return
	}
	if impersonationEnabled {
		serviceAccountToImpersonate, err := settings.DeriveServiceAccountToImpersonate(project, app, destCluster)
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
		sync.WithPermissionValidator(func(un *unstructured.Unstructured, res *metav1.APIResource) error {
			return validateSyncPermissions(project, destCluster, func(proj string) ([]*v1alpha1.Cluster, error) {
				return m.db.GetProjectClusters(context.TODO(), proj)
			}, un, res)
		}),
		sync.WithOperationSettings(syncOp.DryRun, syncOp.Prune, syncOp.SyncStrategy.Force(), syncOp.IsApplyStrategy() || len(syncOp.Resources) > 0),
		sync.WithInitialState(state.Phase, state.Message, initialResourcesRes, state.StartedAt),
		sync.WithResourcesFilter(func(key kube.ResourceKey, target *unstructured.Unstructured, live *unstructured.Unstructured) bool {
			return (len(syncOp.Resources) == 0 ||
				isPostDeleteHook(target) ||
				isPreDeleteHook(target) ||
				argo.ContainsSyncResource(key.Name, key.Namespace, schema.GroupVersionKind{Kind: key.Kind, Group: key.Group}, syncOp.Resources)) &&
				m.isSelfReferencedObj(live, target, app.GetName(), v1alpha1.TrackingMethod(trackingMethod), installationID)
		}),
		sync.WithManifestValidation(!syncOp.SyncOptions.HasOption(common.SyncOptionsDisableValidation)),
		sync.WithSyncWaveHook(delayBetweenSyncWaves),
		sync.WithPruneLast(syncOp.SyncOptions.HasOption(common.SyncOptionPruneLast)),
		sync.WithResourceModificationChecker(syncOp.SyncOptions.HasOption("ApplyOutOfSyncOnly=true"), compareResult.diffResultList),
		sync.WithPrunePropagationPolicy(&prunePropagationPolicy),
		sync.WithReplace(syncOp.SyncOptions.HasOption(common.SyncOptionReplace)),
		sync.WithServerSideApply(syncOp.SyncOptions.HasOption(common.SyncOptionServerSideApply)),
		sync.WithServerSideApplyManager(cdcommon.ArgoCDSSAManager),
		sync.WithClientSideApplyMigration(
			!syncOp.SyncOptions.HasOption(common.SyncOptionDisableClientSideApplyMigration),
			clientSideApplyManager,
		),
		sync.WithPruneConfirmed(app.IsDeletionConfirmed(state.StartedAt.Time)),
		sync.WithDefaultPruneOption(syncOp.SyncOptions.GetOptionValue(common.SyncOptionPrune)),
		sync.WithSkipDryRunOnMissingResource(syncOp.SyncOptions.HasOption(common.SyncOptionSkipDryRunOnMissingResource)),
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
				_, apiVersion, err = m.liveStateCache.GetVersionsInfo(destCluster)
				if err != nil {
					return nil, fmt.Errorf("failed to get version info from the target cluster %q", destCluster.Server)
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
			Images:    res.Images,
		})
	}

	logEntry.WithField("duration", time.Since(start)).Info("sync/terminate complete")

	if !syncOp.DryRun && len(syncOp.Resources) == 0 && state.Phase.Successful() {
		err := m.persistRevisionHistory(app, compareResult.syncStatus.Revision, compareResult.syncStatus.ComparedTo.Source, compareResult.syncStatus.Revisions, compareResult.syncStatus.ComparedTo.Sources, isMultiSourceSync, state.StartedAt, state.Operation.InitiatedBy)
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
func normalizeTargetResources(openAPISchema openapi.Resources, cr *comparisonResult) ([]*unstructured.Unstructured, error) {
	// Normalize live and target resources (cleaning or aligning them)
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
		gvk := normalizedTarget.GroupVersionKind()

		originalTarget := cr.reconciliationResult.Target[idx]
		if live == nil {
			// No live resource, just use target
			patchedTargets = append(patchedTargets, originalTarget)
			continue
		}

		var (
			lookupPatchMeta strategicpatch.LookupPatchMeta
			versionedObject any
		)

		// Load patch meta struct or OpenAPI schema for CRDs
		if versionedObject, err = scheme.Scheme.New(gvk); err == nil {
			if lookupPatchMeta, err = strategicpatch.NewPatchMetaFromStruct(versionedObject); err != nil {
				return nil, err
			}
		} else if crdSchema := openAPISchema.LookupResource(gvk); crdSchema != nil {
			lookupPatchMeta = strategicpatch.NewPatchMetaFromOpenAPI(crdSchema)
		}

		// RespectIgnoreDifferences preserves ignored fields by copying their live
		// values into the target that is applied during sync. `status` must be
		// excluded from that copy: it is owned by the resource's own controller,
		// never by the sync. Merging live `status` into the apply makes the sync
		// field manager (ArgoCDSSAManager, "argocd-controller") a co-owner of
		// `status` under server-side apply. For resources without a /status
		// subresource (e.g. argoproj.io/Application) this freezes a stale
		// status.operationState.phase that the controller can no longer correct.
		liveForPatch, normalizedLiveForPatch := live, normalized.Lives[idx]
		liveForPatch = liveForPatch.DeepCopy()
		unstructured.RemoveNestedField(liveForPatch.Object, "status")

		if normalizedLiveForPatch != nil {
			normalizedLiveForPatch = normalizedLiveForPatch.DeepCopy()
			unstructured.RemoveNestedField(normalizedLiveForPatch.Object, "status")
		}

		livePatch, err := getMergePatch(normalizedLiveForPatch, liveForPatch, lookupPatchMeta)
		if err != nil {
			return nil, err
		}

		// Apply the patch to the normalized target
		// This ensures ignored fields in live are restored into the target before syncing
		normalizedTarget, err = applyMergePatch(normalizedTarget, livePatch, versionedObject, lookupPatchMeta)
		if err != nil {
			return nil, err
		}
		patchedTargets = append(patchedTargets, normalizedTarget)
	}

	return patchedTargets, nil
}

// getMergePatch calculates and returns the patch between the original and the
// modified unstructures.
func getMergePatch(original, modified *unstructured.Unstructured, lookupPatchMeta strategicpatch.LookupPatchMeta) ([]byte, error) {
	originalJSON, err := original.MarshalJSON()
	if err != nil {
		return nil, err
	}
	modifiedJSON, err := modified.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if lookupPatchMeta != nil {
		patch, err := tryStrategicMergePatch(originalJSON, modifiedJSON, lookupPatchMeta)
		if err == nil {
			return patch, nil
		}
		// Strategic merge patch can fail (and even panic) for resources whose schema
		// has incomplete or nil subschemas for some fields, e.g. CRDs with free-form
		// objects such as the Argo CD Application CRD. This is a known bug in the
		// strategic merge patch handling of k8s.io/apimachinery prior to v0.35
		// (see https://github.com/argoproj/argo-cd/issues/25199). Fall back to a
		// regular JSON merge patch so the sync does not fail; the only downside is
		// that ignoring individual array elements is not applied for that resource.
		log.Warnf("falling back to JSON merge patch, strategic merge patch failed: %v", err)
	}

	return jsonpatch.CreateMergePatch(originalJSON, modifiedJSON)
}

// tryStrategicMergePatch computes a three-way strategic merge patch and recovers
// from any panic originating in the k8s.io/apimachinery strategic merge patch code
// (see https://github.com/argoproj/argo-cd/issues/25199), returning an error
// instead so the caller can fall back to a regular JSON merge patch.
func tryStrategicMergePatch(originalJSON, modifiedJSON []byte, lookupPatchMeta strategicpatch.LookupPatchMeta) (patch []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			patch = nil
			err = fmt.Errorf("recovered from panic in strategic merge patch: %v", r)
		}
	}()
	return strategicpatch.CreateThreeWayMergePatch(modifiedJSON, modifiedJSON, originalJSON, lookupPatchMeta, true)
}

// applyMergePatch will apply the given patch in the obj and return the patched unstructure.
func applyMergePatch(obj *unstructured.Unstructured, patch []byte, versionedObject any, meta strategicpatch.LookupPatchMeta) (*unstructured.Unstructured, error) {
	originalJSON, err := obj.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var patchedJSON []byte
	switch {
	case versionedObject != nil:
		patchedJSON, err = strategicpatch.StrategicMergePatch(originalJSON, patch, versionedObject)
	case meta != nil:
		var originalMap, patchMap map[string]any
		if err := json.Unmarshal(originalJSON, &originalMap); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(patch, &patchMap); err != nil {
			return nil, err
		}

		patchedMap, err := strategicpatch.StrategicMergeMapPatchUsingLookupPatchMeta(originalMap, patchMap, meta)
		if err != nil {
			return nil, err
		}
		patchedJSON, err = json.Marshal(patchedMap)
		if err != nil {
			return nil, err
		}
	default:
		patchedJSON, err = jsonpatch.MergePatch(originalJSON, patch)
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
func delayBetweenSyncWaves(_ common.SyncPhase, _ int, finalWave bool) error {
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

func syncWindowPreventsSync(app *v1alpha1.Application, proj *v1alpha1.AppProject) (bool, error) {
	window := proj.Spec.SyncWindows.Matches(app)
	isManual := false
	var operationStartTime *time.Time
	if app.Status.OperationState != nil {
		isManual = !app.Status.OperationState.Operation.InitiatedBy.Automated
		if !app.Status.OperationState.StartedAt.IsZero() {
			t := app.Status.OperationState.StartedAt.Time
			operationStartTime = &t
		}
	}
	canSync, err := window.CanSync(isManual, operationStartTime)
	if err != nil {
		// prevents sync because sync window has an error
		return true, err
	}
	return !canSync, nil
}

// validateSyncPermissions checks whether the given resource is permitted by the project's
// allow/deny lists and destination rules. It returns an error if the API resource info is nil
// (preventing a nil-pointer panic), if the resource's group/kind is not permitted, or if
// the resource's namespace is not an allowed destination.
func validateSyncPermissions(
	project *v1alpha1.AppProject,
	destCluster *v1alpha1.Cluster,
	getProjectClusters func(string) ([]*v1alpha1.Cluster, error),
	un *unstructured.Unstructured,
	res *metav1.APIResource,
) error {
	if res == nil {
		return fmt.Errorf("failed to get API resource info for %s/%s: unable to verify permissions", un.GroupVersionKind().Group, un.GroupVersionKind().Kind)
	}
	if !project.IsGroupKindNamePermitted(un.GroupVersionKind().GroupKind(), un.GetName(), res.Namespaced) {
		return fmt.Errorf("resource %s:%s is not permitted in project %s", un.GroupVersionKind().Group, un.GroupVersionKind().Kind, project.Name)
	}
	if res.Namespaced {
		permitted, err := project.IsDestinationPermitted(destCluster, un.GetNamespace(), getProjectClusters)
		if err != nil {
			return err
		}

		if !permitted {
			return fmt.Errorf("namespace %v is not permitted in project '%s'", un.GetNamespace(), project.Name)
		}
	}
	return nil
}
