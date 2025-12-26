package controller

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	cdcommon "github.com/argoproj/argo-cd/v3/common"

	gitopsDiff "github.com/argoproj/gitops-engine/pkg/diff"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
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
	"github.com/argoproj/argo-cd/v3/util/glob"
	kubeutil "github.com/argoproj/argo-cd/v3/util/kube"
	logutils "github.com/argoproj/argo-cd/v3/util/log"
	"github.com/argoproj/argo-cd/v3/util/lua"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	// EnvVarSyncWaveDelay is an environment variable which controls the delay in seconds between
	// each sync-wave
	EnvVarSyncWaveDelay = "ARGOCD_SYNC_WAVE_DELAY"

	// serviceAccountDisallowedCharSet contains the characters that are not allowed to be present
	// in a DefaultServiceAccount configured for a DestinationServiceAccount
	serviceAccountDisallowedCharSet = "!*[]{}\\/"
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
	clusterCache, err := m.liveStateCache.GetClusterCache(cluster)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting cluster cache: %w", err)
	}

	rawConfig, err := cluster.RawRestConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting cluster REST config: %w", err)
	}
	ops, cleanup, err := kubeutil.ManageServerSideDiffDryRuns(rawConfig, clusterCache.GetOpenAPISchema(), m.onKubectlRun)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kubectl ResourceOperations: %w", err)
	}

	wrappedOps := &secretNormalizingApplier{
		inner:       ops,
		settingsMgr: m.settingsMgr,
	}

	return wrappedOps, cleanup, nil
}

// secretNormalizingApplier wraps a KubeApplier to normalize Secret data
type secretNormalizingApplier struct {
	inner       gitopsDiff.KubeApplier
	settingsMgr *settings.SettingsManager
}

func (s *secretNormalizingApplier) ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force, validate, serverSideApply bool, manager string) (string, error) {
	result, err := s.inner.ApplyResource(ctx, obj, dryRunStrategy, force, validate, serverSideApply, manager)
	if err != nil {
		return result, err
	}

	resultObj := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(result), resultObj); err != nil {
		return result, err
	}

	// Apply hideSecretData
	if resultObj.GetKind() == kube.SecretKind && resultObj.GroupVersionKind().Group == "" {
		_, normalized, err := gitopsDiff.HideSecretData(nil, resultObj, s.settingsMgr.GetSensitiveAnnotations())
		if err != nil {
			return result, fmt.Errorf("error normalizing secret data: %w", err)
		}

		normalizedBytes, err := json.Marshal(normalized)
		if err != nil {
			return result, fmt.Errorf("error marshaling normalized secret: %w", err)
		}
		return string(normalizedBytes), nil
	}

	return result, nil
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

	openAPISchema, err := m.getOpenAPISchema(destCluster)
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
		serviceAccountToImpersonate, err := deriveServiceAccountToImpersonate(project, app, destCluster)
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
			if !project.IsGroupKindNamePermitted(un.GroupVersionKind().GroupKind(), un.GetName(), res.Namespaced) {
				return fmt.Errorf("resource %s:%s is not permitted in project %s", un.GroupVersionKind().Group, un.GroupVersionKind().Kind, project.Name)
			}
			if res.Namespaced {
				permitted, err := project.IsDestinationPermitted(destCluster, un.GetNamespace(), func(project string) ([]*v1alpha1.Cluster, error) {
					return m.db.GetProjectClusters(context.TODO(), project)
				})
				if err != nil {
					return err
				}

				if !permitted {
					return fmt.Errorf("namespace %v is not permitted in project '%s'", un.GetNamespace(), project.Name)
				}
			}
			return nil
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
func applyMergePatch(obj *unstructured.Unstructured, patch []byte, versionedObject any) (*unstructured.Unstructured, error) {
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
	if app.Status.OperationState != nil {
		isManual = !app.Status.OperationState.Operation.InitiatedBy.Automated
	}
	canSync, err := window.CanSync(isManual)
	if err != nil {
		// prevents sync because sync window has an error
		return true, err
	}
	return !canSync, nil
}

// deriveServiceAccountToImpersonate determines the service account to be used for impersonation for the sync operation.
// The returned service account will be fully qualified including namespace and the service account name in the format system:serviceaccount:<namespace>:<service_account>
func deriveServiceAccountToImpersonate(project *v1alpha1.AppProject, application *v1alpha1.Application, destCluster *v1alpha1.Cluster) (string, error) {
	// spec.Destination.Namespace is optional. If not specified, use the Application's
	// namespace
	serviceAccountNamespace := application.Spec.Destination.Namespace
	if serviceAccountNamespace == "" {
		serviceAccountNamespace = application.Namespace
	}
	// Loop through the destinationServiceAccounts and see if there is any destination that is a candidate.
	// if so, return the service account specified for that destination.
	for _, item := range project.Spec.DestinationServiceAccounts {
		dstServerMatched, err := glob.MatchWithError(item.Server, destCluster.Server)
		if err != nil {
			return "", fmt.Errorf("invalid glob pattern for destination server: %w", err)
		}
		dstNamespaceMatched, err := glob.MatchWithError(item.Namespace, application.Spec.Destination.Namespace)
		if err != nil {
			return "", fmt.Errorf("invalid glob pattern for destination namespace: %w", err)
		}
		if dstServerMatched && dstNamespaceMatched {
			if strings.Trim(item.DefaultServiceAccount, " ") == "" || strings.ContainsAny(item.DefaultServiceAccount, serviceAccountDisallowedCharSet) {
				return "", fmt.Errorf("default service account contains invalid chars '%s'", item.DefaultServiceAccount)
			} else if strings.Contains(item.DefaultServiceAccount, ":") {
				// service account is specified along with its namespace.
				return "system:serviceaccount:" + item.DefaultServiceAccount, nil
			}
			// service account needs to be prefixed with a namespace
			return fmt.Sprintf("system:serviceaccount:%s:%s", serviceAccountNamespace, item.DefaultServiceAccount), nil
		}
	}
	// if there is no match found in the AppProject.Spec.DestinationServiceAccounts, use the default service account of the destination namespace.
	return "", fmt.Errorf("no matching service account found for destination server %s and namespace %s", application.Spec.Destination.Server, serviceAccountNamespace)
}
