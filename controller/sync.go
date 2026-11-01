package controller

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"strconv"
	"time"

	cdcommon "github.com/argoproj/argo-cd/v3/common"

	gitopsDiff "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/scheme"
	log "github.com/sirupsen/logrus"
	otel_codes "go.opentelemetry.io/otel/codes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/client-go/rest"

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
func (m *appStateManager) getServerSideDiffDryRunApplier(cluster *v1alpha1.Cluster, project *v1alpha1.AppProject, app *v1alpha1.Application) (gitopsDiff.KubeApplier, func(), error) {
	rawConfig, err := cluster.RawRestConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting cluster REST config: %w", err)
	}
	// When impersonation is enabled, the dry-run apply is executed as the ServiceAccount
	// derived from the AppProject's destinationServiceAccounts, matching the identity used
	// by the sync operation.
	if err := m.applyDiffImpersonationConfig(rawConfig, project, app, cluster); err != nil {
		return nil, nil, err
	}
	ops, cleanup, err := kubeutil.ManageServerSideDiffDryRuns(rawConfig, m.onKubectlRun)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kubectl ResourceOperations: %w", err)
	}
	return ops, cleanup, nil
}

// applyDiffImpersonationConfig sets the impersonation headers on the given REST config
// so that the server-side diff dry-run runs as the same ServiceAccount used by sync.
// It mirrors the sync path's behaviour, including honoring the impersonation-enforced
// setting: when impersonation is enabled but no matching destinationServiceAccount is
// found, enforcement causes the diff to fail rather than silently fall back to the
// controller credential (the dry-run apply is authorized by the API server as a patch,
// so falling back would require the standing cluster credential to hold patch authority).
func (m *appStateManager) applyDiffImpersonationConfig(config *rest.Config, project *v1alpha1.AppProject, app *v1alpha1.Application, destCluster *v1alpha1.Cluster) error {
	logEntry := log.WithFields(applog.GetAppLogFields(app))

	impersonationEnabled, err := m.settingsMgr.IsImpersonationEnabled()
	if err != nil {
		return fmt.Errorf("error getting impersonation setting: %w", err)
	}
	if !impersonationEnabled {
		return nil
	}
	serviceAccountToImpersonate, err := settings.DeriveServiceAccountToImpersonate(project, app, destCluster)
	if err != nil {
		return fmt.Errorf("error deriving service account to impersonate: %w", err)
	}
	if serviceAccountToImpersonate == "" {
		// No matching service account found - check enforcement.
		impersonationEnforced, err := m.settingsMgr.IsImpersonationEnforced()
		if err != nil {
			return fmt.Errorf("error getting impersonation enforcement setting: %w", err)
		}
		if impersonationEnforced {
			return fmt.Errorf("no matching service account found for destination server %s and namespace %s", destCluster.Server, app.Spec.Destination.Namespace)
		}
		// Non-enforced mode: fall back to the controller credential, consistent with sync.
		logEntry.Debugf("server-side diff: no matching service account found for impersonation (project: %s, server: %s, namespace: %s), falling back to controller service account", project.Name, destCluster.Server, app.Spec.Destination.Namespace)
		return nil
	}
	logEntry.Debugf("server-side diff: impersonating service account %q, matching sync", serviceAccountToImpersonate)
	config.Impersonate = rest.ImpersonationConfig{
		UserName: serviceAccountToImpersonate,
	}
	return nil
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

func (m *appStateManager) SyncAppState(ctx context.Context, app *v1alpha1.Application, project *v1alpha1.AppProject, state *v1alpha1.OperationState) {
	ctx, span := tracer.Start(ctx, "controller.SyncAppState")
	setAppTraceAttrs(span, app)
	// SyncAppState is void; it signals failure through state.Phase rather than a return value, so
	// map a terminal failed phase onto the span status at exit (mirroring traceutil.EndSpan).
	defer func() {
		if state.Phase.Failed() {
			span.SetStatus(otel_codes.Error, state.Message)
		}
		span.End()
	}()
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

	// A terminate request must not be blocked by a sync window. Otherwise an operation that is
	// no longer allowed to sync by its window can never be terminated and stays in the
	// terminating phase forever.
	if isBlocked, err := syncWindowPreventsSync(app, project); isBlocked && state.Phase != common.OperationTerminating {
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
	compareResult, err := m.CompareAppState(ctx, app, project, revisions, sources, false, true, syncOp.Manifests, isMultiSourceSync)
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

	destCluster, err := argo.GetDestinationCluster(ctx, app.Spec.Destination, m.db)
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
		serviceAccountToImpersonate, err := settings.DeriveServiceAccountToImpersonate(project, app, destCluster)
		if err != nil {
			state.Phase = common.OperationError
			state.Message = fmt.Sprintf("failed to derive service account to impersonate: %v", err)
			return
		}

		if serviceAccountToImpersonate == "" {
			// No matching service account found - check enforcement
			impersonationEnforced, enforcedErr := m.settingsMgr.IsImpersonationEnforced()
			if enforcedErr != nil {
				log.Errorf("could not get impersonation enforcement flag: %v", enforcedErr)
				state.Phase = common.OperationError
				state.Message = fmt.Sprintf("failed to check impersonation enforcement setting: %v", enforcedErr)
				return
			}

			if impersonationEnforced {
				state.Phase = common.OperationError
				state.Message = fmt.Sprintf("no matching service account found for destination server %s and namespace %s", destCluster.Server, app.Spec.Destination.Namespace)
				return
			}

			// Non-enforced mode: log info and continue with controller SA
			logEntry.Infof("no matching service account found for impersonation (project: %s, server: %s, namespace: %s), falling back to controller service account", project.Name, destCluster.Server, app.Spec.Destination.Namespace)
		} else {
			logEntry = logEntry.WithFields(log.Fields{"impersonationEnabled": "true", "serviceAccount": serviceAccountToImpersonate})
			// set the impersonation headers.
			rawConfig.Impersonate = rest.ImpersonationConfig{
				UserName: serviceAccountToImpersonate,
			}
			restConfig.Impersonate = rest.ImpersonationConfig{
				UserName: serviceAccountToImpersonate,
			}
		}
	}

	opts := []sync.SyncOpt{
		sync.WithLogr(logutils.NewLogrusLogger(logEntry)),
		sync.WithHealthOverride(lua.ResourceHealthOverrides(resourceOverrides)),
		sync.WithPermissionValidator(func(un *unstructured.Unstructured, res *metav1.APIResource) error {
			return validateSyncPermissions(project, destCluster, func(proj string) ([]*v1alpha1.Cluster, error) {
				return m.db.GetProjectClusters(ctx, proj)
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
		syncCtx.Terminate(ctx)
	} else {
		syncCtx.Sync(ctx)
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

// normalizeTargetResources builds the objects a RespectIgnoreDifferences sync
// applies: for each target, the git manifest everywhere except at the paths
// the application's ignore rules name. What it does:
//
//  1. Only the ignore rules (jsonPointers, jqPathExpressions,
//     managedFieldsManagers) decide what is ignored. The other diff
//     normalizers (known types, tracking) never change what is applied.
//  2. A resource with no ignored path is applied exactly as given.
//  3. A path a rule removes from git is not asserted.
//  4. A path a rule removes from live keeps its live value. Where git still
//     asserts a value at that path, git wins.
//  5. List items are matched by the schema's merge keys, with missing key
//     fields taking their schema defaults; a list without keys is matched by
//     position. A live-only item is inserted at its live position. A field
//     inside an item git does not declare is not carried: a field rule never
//     keeps an item, so deleting the item from git deletes it.
//  6. An atomic map or list git does not declare is never created from a
//     partial set of ignored keys; a whole ignored value is carried as is.
//  7. status and the metadata fields the API server populates are never
//     carried, and the application's tracking annotation and label are
//     always asserted from the manifest.
//  8. A Secret's stringData is folded into data before the rules run.
//  9. The resource's apiVersion, kind, name and namespace are always the
//     manifest's. A rule that cannot be evaluated (not a path expression, a
//     compile error, a timeout), a rule that indexes a keyed list where the
//     manifest and the cluster have different items at that index, a rule
//     that names a merge key, and a managedFieldsManagers rule that trusts the
//     sync's own manager fail the sync with the rule named.
//
// The schema is the structured-merge-diff schema of the cluster, the one
// server-side apply uses; for a kind without one, lists are atomic and maps
// granular.
//
// Every rule is resolved against the original objects. The diff normalizer
// instead applies rules one after another, so when one rule removes a list
// item and a later rule indexes into the same list, the diff addresses the
// item that shifted into that index while the sync addresses the item the
// rule names. Rules that select items by content are unaffected.
//
// The guarantees that follow from this process are listed at the top of
// sync_respect_ignore_semantics_test.go, one test each, with the reported
// shapes as examples.
func normalizeTargetResources(cr *comparisonResult) ([]*unstructured.Unstructured, error) {
	resolver, err := diff.NewIgnoredPathsResolver(cr.reconciliationResult.Target, cr.diffConfig)
	if err != nil {
		return nil, err
	}

	patchedTargets := []*unstructured.Unstructured{}

	for idx, live := range cr.reconciliationResult.Live {
		target := cr.reconciliationResult.Target[idx]
		if target == nil {
			patchedTargets = append(patchedTargets, nil)
			continue
		}
		if live == nil {
			// No live resource, just use target
			patchedTargets = append(patchedTargets, target)
			continue
		}

		// The API server folds stringData into data; do the same so ignore
		// rules on data keys apply to what the server will store.
		prepared := foldSecretStringData(target)

		livePaths, targetPaths, err := resolver.IgnoredPaths(live, prepared)
		if err != nil {
			return nil, err
		}
		stringDataLive, stringDataTarget, err := stringDataPaths(resolver, target)
		if err != nil {
			return nil, err
		}
		livePaths = append(livePaths, stringDataLive...)
		targetPaths = append(targetPaths, stringDataTarget...)
		if len(livePaths) == 0 && len(targetPaths) == 0 {
			// Nothing is ignored on this resource: apply the manifest as is.
			patchedTargets = append(patchedTargets, target)
			continue
		}
		pt := scheme.ResolveParseableType(prepared.GroupVersionKind(), cr.diffConfig.GVKParser())
		pointerPaths, err := resolver.PointerPaths(prepared)
		if err != nil {
			return nil, err
		}
		if err := checkIndexPaths(live.Object, prepared.Object, pointerPaths, newFieldSchema(pt)); err != nil {
			return nil, fmt.Errorf("%s %s/%s: %w", target.GetKind(), target.GetNamespace(), target.GetName(), err)
		}
		patched, err := buildApplyTarget(live, prepared, livePaths, targetPaths, newFieldSchema(pt))
		if err != nil {
			return nil, err
		}
		restoreTrackingIdentity(patched, target, cr.diffConfig.AppLabelKey(), cr.diffConfig.TrackingMethod())
		patchedTargets = append(patchedTargets, patched)
	}
	return patchedTargets, nil
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
