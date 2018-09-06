package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/apis/batch"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/kube"
)

type syncContext struct {
	appName       string
	comparison    *appv1.ComparisonResult
	config        *rest.Config
	dynClientPool dynamic.ClientPool
	disco         *discovery.DiscoveryClient
	kubectl       kube.Kubectl
	namespace     string
	syncOp        *appv1.SyncOperation
	syncRes       *appv1.SyncOperationResult
	opState       *appv1.OperationState
	manifestInfo  *repository.ManifestResponse
	log           *log.Entry
	// lock to protect concurrent updates of the result list
	lock sync.Mutex
}

func (s *ksonnetAppStateManager) SyncAppState(app *appv1.Application, state *appv1.OperationState) {
	// Sync requests are usually requested with ambiguous revisions (e.g. master, HEAD, v1.2.3).
	// This can change meaning when resuming operations (e.g a hook sync). After calculating a
	// concrete git commit SHA, the SHA is remembered in the status.operationState.syncResult and
	// rollbackResult fields. This ensures that when resuming an operation, we sync to the same
	// revision that we initially started with.
	var revision string
	var syncOp appv1.SyncOperation
	var syncRes *appv1.SyncOperationResult
	var overrides []appv1.ComponentParameter

	if state.Operation.Sync != nil {
		syncOp = *state.Operation.Sync
		if state.SyncResult != nil {
			syncRes = state.SyncResult
			revision = state.SyncResult.Revision
		} else {
			syncRes = &appv1.SyncOperationResult{}
			state.SyncResult = syncRes
		}
	} else if state.Operation.Rollback != nil {
		var deploymentInfo *appv1.DeploymentInfo
		for _, info := range app.Status.History {
			if info.ID == app.Operation.Rollback.ID {
				deploymentInfo = &info
				break
			}
		}
		if deploymentInfo == nil {
			state.Phase = appv1.OperationFailed
			state.Message = fmt.Sprintf("application %s does not have deployment with id %v", app.Name, app.Operation.Rollback.ID)
			return
		}
		// Rollback is just a convenience around Sync
		syncOp = appv1.SyncOperation{
			Revision:     deploymentInfo.Revision,
			DryRun:       state.Operation.Rollback.DryRun,
			Prune:        state.Operation.Rollback.Prune,
			SyncStrategy: &appv1.SyncStrategy{Apply: &appv1.SyncStrategyApply{}},
		}
		overrides = deploymentInfo.ComponentParameterOverrides
		if state.RollbackResult != nil {
			syncRes = state.RollbackResult
			revision = state.RollbackResult.Revision
		} else {
			syncRes = &appv1.SyncOperationResult{}
			state.RollbackResult = syncRes
		}
	} else {
		state.Phase = appv1.OperationFailed
		state.Message = "Invalid operation request: no operation specified"
		return
	}

	if revision == "" {
		// if we get here, it means we did not remember a commit SHA which we should be syncing to.
		// This typically indicates we are just about to begin a brand new sync/rollback operation.
		// Take the value in the requested operation. We will resolve this to a SHA later.
		revision = syncOp.Revision
	}
	comparison, manifestInfo, conditions, err := s.CompareAppState(app, revision, overrides)
	if err != nil {
		state.Phase = appv1.OperationError
		state.Message = err.Error()
		return
	}
	errConditions := make([]appv1.ApplicationCondition, 0)
	for i := range conditions {
		if conditions[i].IsError() {
			errConditions = append(errConditions, conditions[i])
		}
	}
	if len(errConditions) > 0 {
		state.Phase = appv1.OperationError
		state.Message = argo.FormatAppConditions(errConditions)
		return
	}
	// We now have a concrete commit SHA. Set this in the sync result revision so that we remember
	// what we should be syncing to when resuming operations.
	syncRes.Revision = manifestInfo.Revision

	clst, err := s.db.GetCluster(context.Background(), app.Spec.Destination.Server)
	if err != nil {
		state.Phase = appv1.OperationError
		state.Message = err.Error()
		return
	}

	restConfig := clst.RESTConfig()
	dynClientPool := dynamic.NewDynamicClientPool(restConfig)
	disco, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		state.Phase = appv1.OperationError
		state.Message = fmt.Sprintf("Failed to initialize dynamic client: %v", err)
		return
	}

	syncCtx := syncContext{
		appName:       app.Name,
		comparison:    comparison,
		config:        restConfig,
		dynClientPool: dynClientPool,
		disco:         disco,
		kubectl:       s.kubectl,
		namespace:     app.Spec.Destination.Namespace,
		syncOp:        &syncOp,
		syncRes:       syncRes,
		opState:       state,
		manifestInfo:  manifestInfo,
		log:           log.WithFields(log.Fields{"application": app.Name}),
	}

	if state.Phase == appv1.OperationTerminating {
		syncCtx.terminate()
	} else {
		syncCtx.sync()
	}

	if !syncOp.DryRun && syncCtx.opState.Phase.Successful() {
		err := s.persistDeploymentInfo(app, manifestInfo.Revision, manifestInfo.Params, nil)
		if err != nil {
			state.Phase = appv1.OperationError
			state.Message = fmt.Sprintf("failed to record sync to history: %v", err)
		}
	}
}

// syncTask holds the live and target object. At least one should be non-nil. A targetObj of nil
// indicates the live object needs to be pruned. A liveObj of nil indicates the object has yet to
// be deployed
type syncTask struct {
	liveObj   *unstructured.Unstructured
	targetObj *unstructured.Unstructured
}

// sync has performs the actual apply or hook based sync
func (sc *syncContext) sync() {
	syncTasks, successful := sc.generateSyncTasks()
	if !successful {
		return
	}
	// Perform a `kubectl apply --dry-run` against all the manifests. This will detect most (but
	// not all) validation issues with the user's manifests (e.g. will detect syntax issues, but
	// will not not detect if they are mutating immutable fields). If anything fails, we will refuse
	// to perform the sync.
	if !sc.startedPreSyncPhase() {
		// Optimization: we only wish to do this once per operation, performing additional dry-runs
		// is harmless, but redundant. The indicator we use to detect if we have already performed
		// the dry-run for this operation, is if the resource or hook list is empty.
		if !sc.doApplySync(syncTasks, true, false, sc.syncOp.DryRun) {
			sc.setOperationPhase(appv1.OperationFailed, "one or more objects failed to apply (dry run)")
			return
		}
		if sc.syncOp.DryRun {
			sc.setOperationPhase(appv1.OperationSucceeded, "successfully synced (dry run)")
			return
		}
	}

	// All objects passed a `kubectl apply --dry-run`, so we are now ready to actually perform the sync.
	if sc.syncOp.SyncStrategy == nil {
		// default sync strategy to hook if no strategy
		sc.syncOp.SyncStrategy = &appv1.SyncStrategy{Hook: &appv1.SyncStrategyHook{}}
	}
	if sc.syncOp.SyncStrategy.Apply != nil {
		if !sc.startedSyncPhase() {
			if !sc.doApplySync(syncTasks, false, sc.syncOp.SyncStrategy.Apply.Force, true) {
				sc.setOperationPhase(appv1.OperationFailed, "one or more objects failed to apply")
				return
			}
			// If apply was successful, return here and force an app refresh. This is so the app
			// will become requeued into the workqueue, to force a new sync/health assessment before
			// marking the operation as completed
			return
		}
		sc.setOperationPhase(appv1.OperationSucceeded, "successfully synced")
	} else if sc.syncOp.SyncStrategy.Hook != nil {
		hooks, err := sc.getHooks()
		if err != nil {
			sc.setOperationPhase(appv1.OperationError, fmt.Sprintf("failed to generate hooks resources: %v", err))
			return
		}
		sc.doHookSync(syncTasks, hooks)
	} else {
		sc.setOperationPhase(appv1.OperationFailed, "Unknown sync strategy")
		return
	}
}

func (sc *syncContext) forceAppRefresh() {
	sc.comparison.ComparedAt = metav1.Time{}
}

// generateSyncTasks() generates the list of sync tasks we will be performing during this sync.
func (sc *syncContext) generateSyncTasks() ([]syncTask, bool) {
	syncTasks := make([]syncTask, 0)
	for _, resourceState := range sc.comparison.Resources {
		liveObj, err := resourceState.LiveObject()
		if err != nil {
			sc.setOperationPhase(appv1.OperationError, fmt.Sprintf("Failed to unmarshal live object: %v", err))
			return nil, false
		}
		targetObj, err := resourceState.TargetObject()
		if err != nil {
			sc.setOperationPhase(appv1.OperationError, fmt.Sprintf("Failed to unmarshal target object: %v", err))
			return nil, false
		}
		syncTask := syncTask{
			liveObj:   liveObj,
			targetObj: targetObj,
		}
		syncTasks = append(syncTasks, syncTask)
	}
	return syncTasks, true
}

// startedPreSyncPhase detects if we already started the PreSync stage of a sync operation.
// This is equal to if we have anything in our resource or hook list
func (sc *syncContext) startedPreSyncPhase() bool {
	if len(sc.syncRes.Resources) > 0 {
		return true
	}
	if len(sc.syncRes.Hooks) > 0 {
		return true
	}
	return false
}

// startedSyncPhase detects if we have already started the Sync stage of a sync operation.
// This is equal to if the resource list is non-empty, or we we see Sync/PostSync hooks
func (sc *syncContext) startedSyncPhase() bool {
	if len(sc.syncRes.Resources) > 0 {
		return true
	}
	for _, hookStatus := range sc.syncRes.Hooks {
		if hookStatus.Type == appv1.HookTypeSync || hookStatus.Type == appv1.HookTypePostSync {
			return true
		}
	}
	return false
}

// startedPostSyncPhase detects if we have already started the PostSync stage. This is equal to if
// we see any PostSync hooks
func (sc *syncContext) startedPostSyncPhase() bool {
	for _, hookStatus := range sc.syncRes.Hooks {
		if hookStatus.Type == appv1.HookTypePostSync {
			return true
		}
	}
	return false
}

func (sc *syncContext) setOperationPhase(phase appv1.OperationPhase, message string) {
	if sc.opState.Phase != phase || sc.opState.Message != message {
		sc.log.Infof("Updating operation state. phase: %s -> %s, message: '%s' -> '%s'", sc.opState.Phase, phase, sc.opState.Message, message)
	}
	sc.opState.Phase = phase
	sc.opState.Message = message
}

// applyObject performs a `kubectl apply` of a single resource
func (sc *syncContext) applyObject(targetObj *unstructured.Unstructured, dryRun bool, force bool) appv1.ResourceDetails {
	resDetails := appv1.ResourceDetails{
		Name:      targetObj.GetName(),
		Kind:      targetObj.GetKind(),
		Namespace: sc.namespace,
	}
	message, err := sc.kubectl.ApplyResource(sc.config, targetObj, sc.namespace, dryRun, force)
	if err != nil {
		resDetails.Message = err.Error()
		resDetails.Status = appv1.ResourceDetailsSyncFailed
		return resDetails
	}
	resDetails.Message = message
	resDetails.Status = appv1.ResourceDetailsSynced
	return resDetails
}

// pruneObject deletes the object if both prune is true and dryRun is false. Otherwise appropriate message
func (sc *syncContext) pruneObject(liveObj *unstructured.Unstructured, prune, dryRun bool) appv1.ResourceDetails {
	resDetails := appv1.ResourceDetails{
		Name:      liveObj.GetName(),
		Kind:      liveObj.GetKind(),
		Namespace: liveObj.GetNamespace(),
	}
	if prune {
		if dryRun {
			resDetails.Message = "pruned (dry run)"
			resDetails.Status = appv1.ResourceDetailsSyncedAndPruned
		} else {
			err := sc.kubectl.DeleteResource(sc.config, liveObj, sc.namespace)
			if err != nil {
				resDetails.Message = err.Error()
				resDetails.Status = appv1.ResourceDetailsSyncFailed
			} else {
				resDetails.Message = "pruned"
				resDetails.Status = appv1.ResourceDetailsSyncedAndPruned
			}
		}
	} else {
		resDetails.Message = "ignored (requires pruning)"
		resDetails.Status = appv1.ResourceDetailsPruningRequired
	}
	return resDetails
}

// performs a apply based sync of the given sync tasks (possibly pruning the objects).
// If update is true, will updates the resource details with the result.
// Or if the prune/apply failed, will also update the result.
func (sc *syncContext) doApplySync(syncTasks []syncTask, dryRun, force, update bool) bool {
	syncSuccessful := true
	// apply all resources in parallel
	var wg sync.WaitGroup
	for _, task := range syncTasks {
		wg.Add(1)
		go func(t syncTask) {
			defer wg.Done()
			var resDetails appv1.ResourceDetails
			if t.targetObj == nil {
				resDetails = sc.pruneObject(t.liveObj, sc.syncOp.Prune, dryRun)
			} else {
				if isHook(t.targetObj) {
					return
				}
				resDetails = sc.applyObject(t.targetObj, dryRun, force)
			}
			if !resDetails.Status.Successful() {
				syncSuccessful = false
			}
			if update || !resDetails.Status.Successful() {
				sc.setResourceDetails(&resDetails)
			}
		}(task)
	}
	wg.Wait()
	return syncSuccessful
}

// doHookSync initiates (or continues) a hook-based sync. This method will be invoked when there may
// already be in-flight (potentially incomplete) jobs/workflows, and should be idempotent.
func (sc *syncContext) doHookSync(syncTasks []syncTask, hooks []*unstructured.Unstructured) {
	// 1. Run PreSync hooks
	if !sc.runHooks(hooks, appv1.HookTypePreSync) {
		return
	}

	// 2. Run Sync hooks (e.g. blue-green sync workflow)
	// Before performing Sync hooks, apply any normal manifests which aren't annotated with a hook.
	// We only want to do this once per operation.
	shouldContinue := true
	if !sc.startedSyncPhase() {
		if !sc.syncNonHookTasks(syncTasks) {
			sc.setOperationPhase(appv1.OperationFailed, "one or more objects failed to apply")
			return
		}
		shouldContinue = false
	}
	if !sc.runHooks(hooks, appv1.HookTypeSync) {
		shouldContinue = false
	}
	if !shouldContinue {
		return
	}

	// 3. Run PostSync hooks
	// Before running PostSync hooks, we want to make rollout is complete (app is healthy). If we
	// already started the post-sync phase, then we do not need to perform the health check.
	postSyncHooks, _ := sc.getHooks(appv1.HookTypePostSync)
	if len(postSyncHooks) > 0 && !sc.startedPostSyncPhase() {
		healthState, err := setApplicationHealth(sc.kubectl, sc.comparison)
		sc.log.Infof("PostSync application health check: %s", healthState.Status)
		if err != nil {
			sc.setOperationPhase(appv1.OperationError, fmt.Sprintf("failed to check application health: %v", err))
			return
		}
		if healthState.Status != appv1.HealthStatusHealthy {
			sc.setOperationPhase(appv1.OperationRunning, fmt.Sprintf("waiting for %s state to run %s hooks (current health: %s)", appv1.HealthStatusHealthy, appv1.HookTypePostSync, healthState.Status))
			return
		}
	}
	if !sc.runHooks(hooks, appv1.HookTypePostSync) {
		return
	}

	// if we get here, all hooks successfully completed
	sc.setOperationPhase(appv1.OperationSucceeded, "successfully synced")
}

// getHooks returns all ArgoCD hooks, optionally filtered by ones of the specific type(s)
func (sc *syncContext) getHooks(hookTypes ...appv1.HookType) ([]*unstructured.Unstructured, error) {
	var hooks []*unstructured.Unstructured
	for _, manifest := range sc.manifestInfo.Manifests {
		var hook unstructured.Unstructured
		err := json.Unmarshal([]byte(manifest), &hook)
		if err != nil {
			return nil, err
		}
		if !isArgoHook(&hook) {
			// TODO: in the future, if we want to map helm hooks to ArgoCD lifecycles, we should
			// include helm hooks in the returned list
			continue
		}
		if len(hookTypes) > 0 {
			match := false
			for _, desiredType := range hookTypes {
				if isHookType(&hook, desiredType) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		hooks = append(hooks, &hook)
	}
	return hooks, nil
}

// runHooks iterates & filters the target manifests for resources of the specified hook type, then
// creates the resource. Updates the sc.opRes.hooks with the current status. Returns whether or not
// we should continue to the next hook phase.
func (sc *syncContext) runHooks(hooks []*unstructured.Unstructured, hookType appv1.HookType) bool {
	shouldContinue := true
	for _, hook := range hooks {
		if hookType == appv1.HookTypeSync && isHookType(hook, appv1.HookTypeSkip) {
			// If we get here, we are invoking all sync hooks and reached a resource that is
			// annotated with the Skip hook. This will update the resource details to indicate it
			// was skipped due to annotation
			sc.setResourceDetails(&appv1.ResourceDetails{
				Name:      hook.GetName(),
				Kind:      hook.GetKind(),
				Namespace: sc.namespace,
				Message:   "Skipped",
			})
			continue
		}
		if !isHookType(hook, hookType) {
			continue
		}
		updated, err := sc.runHook(hook, hookType)
		if err != nil {
			sc.setOperationPhase(appv1.OperationError, fmt.Sprintf("%s hook error: %v", hookType, err))
			return false
		}
		if updated {
			// If the result of running a hook, caused us to modify hook resource state, we should
			// not proceed to the next hook phase. This is because before proceeding to the next
			// phase, we want a full health assessment to happen. By returning early, we allow
			// the application to get requeued into the controller workqueue, and on the next
			// process iteration, a new CompareAppState() will be performed to get the most
			// up-to-date live state. This enables us to accurately wait for an application to
			// become Healthy before proceeding to run PostSync tasks.
			shouldContinue = false
		}
	}
	if !shouldContinue {
		sc.log.Infof("Stopping after %s phase due to modifications to hook resource state", hookType)
		return false
	}
	completed, successful := areHooksCompletedSuccessful(hookType, sc.syncRes.Hooks)
	if !completed {
		return false
	}
	if !successful {
		sc.setOperationPhase(appv1.OperationFailed, fmt.Sprintf("%s hook failed", hookType))
		return false
	}
	return true
}

// runHook runs the supplied hook and updates the hook status. Returns true if the result of
// invoking this method resulted in changes to any hook status
func (sc *syncContext) runHook(hook *unstructured.Unstructured, hookType appv1.HookType) (bool, error) {
	// Hook resources names are deterministic, whether they are defined by the user (metadata.name),
	// or formulated at the time of the operation (metadata.generateName). If user specifies
	// metadata.generateName, then we will generate a formulated metadata.name before submission.
	if hook.GetName() == "" {
		postfix := strings.ToLower(fmt.Sprintf("%s-%s-%d", sc.syncRes.Revision[0:7], hookType, sc.opState.StartedAt.UTC().Unix()))
		generatedName := hook.GetGenerateName()
		hook = hook.DeepCopy()
		hook.SetName(fmt.Sprintf("%s%s", generatedName, postfix))
	}
	// Check our hook statuses to see if we already completed this hook.
	// If so, this method is a noop
	prevStatus := sc.getHookStatus(hook, hookType)
	if prevStatus != nil && prevStatus.Status.Completed() {
		return false, nil
	}

	gvk := hook.GroupVersionKind()
	dclient, err := sc.dynClientPool.ClientForGroupVersionKind(gvk)
	if err != nil {
		return false, err
	}
	apiResource, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)
	if err != nil {
		return false, err
	}
	resIf := dclient.Resource(apiResource, sc.namespace)

	var liveObj *unstructured.Unstructured
	existing, err := resIf.Get(hook.GetName(), metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			return false, fmt.Errorf("Failed to get status of %s hook %s '%s': %v", hookType, gvk, hook.GetName(), err)
		}
		hook = hook.DeepCopy()
		err = kube.SetLabel(hook, common.LabelApplicationName, sc.appName)
		if err != nil {
			sc.log.Warnf("Failed to set application label on hook %v: %v", hook, err)
		}
		_, err := sc.kubectl.ApplyResource(sc.config, hook, sc.namespace, false, false)
		if err != nil {
			return false, fmt.Errorf("Failed to create %s hook %s '%s': %v", hookType, gvk, hook.GetName(), err)
		}
		created, err := resIf.Get(hook.GetName(), metav1.GetOptions{})
		if err != nil {
			return true, fmt.Errorf("Failed to get status of %s hook %s '%s': %v", hookType, gvk, hook.GetName(), err)
		}
		sc.log.Infof("%s hook %s '%s' created", hookType, gvk, created.GetName())
		sc.setOperationPhase(appv1.OperationRunning, fmt.Sprintf("running %s hooks", hookType))
		liveObj = created
	} else {
		liveObj = existing
	}
	hookStatus := newHookStatus(liveObj, hookType)
	if hookStatus.Status.Completed() {
		if enforceDeletePolicy(hook, hookStatus.Status) {
			err = sc.deleteHook(hook.GetName(), hook.GetKind(), hook.GetAPIVersion())
			if err != nil {
				hookStatus.Status = appv1.OperationFailed
				hookStatus.Message = fmt.Sprintf("failed to delete %s hook: %v", hookStatus.Status, err)
			}
		}
	}
	return sc.updateHookStatus(hookStatus), nil
}

// enforceDeletePolicy examines the hook deletion policy of a object and deletes it based on the status
func enforceDeletePolicy(hook *unstructured.Unstructured, phase appv1.OperationPhase) bool {
	annotations := hook.GetAnnotations()
	if annotations == nil {
		return false
	}
	deletePolicies := strings.Split(annotations[common.AnnotationHookDeletePolicy], ",")
	for _, dp := range deletePolicies {
		policy := appv1.HookDeletePolicy(strings.TrimSpace(dp))
		if policy == appv1.HookDeletePolicyHookSucceeded && phase == appv1.OperationSucceeded {
			return true
		}
		if policy == appv1.HookDeletePolicyHookFailed && phase == appv1.OperationFailed {
			return true
		}
	}
	return false
}

// isHookType tells whether or not the supplied object is a hook of the specified type
func isHookType(hook *unstructured.Unstructured, hookType appv1.HookType) bool {
	annotations := hook.GetAnnotations()
	if annotations == nil {
		return false
	}
	resHookTypes := strings.Split(annotations[common.AnnotationHook], ",")
	for _, ht := range resHookTypes {
		if string(hookType) == strings.TrimSpace(ht) {
			return true
		}
	}
	return false
}

// isHook indicates if the object is either a ArgoCD or Helm hook
func isHook(obj *unstructured.Unstructured) bool {
	return isArgoHook(obj) || isHelmHook(obj)
}

// isHelmHook indicates if the supplied object is a helm hook
func isHelmHook(obj *unstructured.Unstructured) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[common.AnnotationHook]
	return ok
}

// isArgoHook indicates if the supplied object is an ArgoCD application lifecycle hook
// (vs. a normal, synced application resource)
func isArgoHook(obj *unstructured.Unstructured) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	resHookTypes := strings.Split(annotations[common.AnnotationHook], ",")
	for _, hookType := range resHookTypes {
		hookType = strings.TrimSpace(hookType)
		switch appv1.HookType(hookType) {
		case appv1.HookTypePreSync, appv1.HookTypeSync, appv1.HookTypePostSync:
			return true
		}
	}
	return false
}

// syncNonHookTasks syncs or prunes the objects that are not handled by hooks using an apply sync.
// returns true if the sync was successful
func (sc *syncContext) syncNonHookTasks(syncTasks []syncTask) bool {
	var nonHookTasks []syncTask
	for _, task := range syncTasks {
		if task.targetObj == nil {
			nonHookTasks = append(nonHookTasks, task)
		} else {
			annotations := task.targetObj.GetAnnotations()
			if annotations != nil && annotations[common.AnnotationHook] != "" {
				// we are doing a hook sync and this resource is annotated with a hook annotation
				continue
			}
			// if we get here, this resource does not have any hook annotation so we
			// should perform an `kubectl apply`
			nonHookTasks = append(nonHookTasks, task)
		}
	}
	return sc.doApplySync(nonHookTasks, false, sc.syncOp.SyncStrategy.Hook.Force, true)
}

// setResourceDetails sets a resource details in the SyncResult.Resources list
func (sc *syncContext) setResourceDetails(details *appv1.ResourceDetails) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	for i, res := range sc.syncRes.Resources {
		if res.Kind == details.Kind && res.Name == details.Name {
			// update existing value
			if res.Status != details.Status {
				sc.log.Infof("updated resource %s/%s status: %s -> %s", res.Kind, res.Name, res.Status, details.Status)
			}
			if res.Message != details.Message {
				sc.log.Infof("updated resource %s/%s message: %s -> %s", res.Kind, res.Name, res.Message, details.Message)
			}
			sc.syncRes.Resources[i] = details
			return
		}
	}
	sc.log.Infof("added resource %s/%s status: %s, message: %s", details.Kind, details.Name, details.Status, details.Message)
	sc.syncRes.Resources = append(sc.syncRes.Resources, details)
}

func (sc *syncContext) getHookStatus(hookObj *unstructured.Unstructured, hookType appv1.HookType) *appv1.HookStatus {
	for _, hr := range sc.syncRes.Hooks {
		if hr.Name == hookObj.GetName() && hr.Kind == hookObj.GetKind() && hr.Type == hookType {
			return hr
		}
	}
	return nil
}

func newHookStatus(hook *unstructured.Unstructured, hookType appv1.HookType) appv1.HookStatus {
	hookStatus := appv1.HookStatus{
		Name:       hook.GetName(),
		Kind:       hook.GetKind(),
		APIVersion: hook.GetAPIVersion(),
		Type:       hookType,
	}
	switch hookStatus.Kind {
	case "Job":
		var job batch.Job
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(hook.Object, &job)
		if err != nil {
			hookStatus.Status = appv1.OperationError
			hookStatus.Message = err.Error()
		} else {
			failed := false
			var failMsg string
			complete := false
			var message string
			for _, condition := range job.Status.Conditions {
				switch condition.Type {
				case batch.JobFailed:
					failed = true
					complete = true
					failMsg = condition.Message
				case batch.JobComplete:
					complete = true
					message = condition.Message
				}
			}
			if !complete {
				hookStatus.Status = appv1.OperationRunning
				hookStatus.Message = message
			} else if failed {
				hookStatus.Status = appv1.OperationFailed
				hookStatus.Message = failMsg
			} else {
				hookStatus.Status = appv1.OperationSucceeded
				hookStatus.Message = message
			}
		}
	case "Workflow":
		var wf wfv1.Workflow
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(hook.Object, &wf)
		if err != nil {
			hookStatus.Status = appv1.OperationError
			hookStatus.Message = err.Error()
		} else {
			switch wf.Status.Phase {
			case wfv1.NodeRunning:
				hookStatus.Status = appv1.OperationRunning
			case wfv1.NodeSucceeded:
				hookStatus.Status = appv1.OperationSucceeded
			case wfv1.NodeFailed:
				hookStatus.Status = appv1.OperationFailed
			case wfv1.NodeError:
				hookStatus.Status = appv1.OperationError
			}
			hookStatus.Message = wf.Status.Message
		}
	default:
		hookStatus.Status = appv1.OperationSucceeded
		hookStatus.Message = fmt.Sprintf("%s created", hook.GetName())
	}
	return hookStatus
}

// updateHookStatus updates the status of a hook. Returns true if the hook was modified
func (sc *syncContext) updateHookStatus(hookStatus appv1.HookStatus) bool {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	for i, prev := range sc.syncRes.Hooks {
		if prev.Name == hookStatus.Name && prev.Kind == hookStatus.Kind && prev.Type == hookStatus.Type {
			if reflect.DeepEqual(prev, hookStatus) {
				return false
			}
			if prev.Status != hookStatus.Status {
				sc.log.Infof("Hook %s %s/%s status: %s -> %s", hookStatus.Type, prev.Kind, prev.Name, prev.Status, hookStatus.Status)
			}
			if prev.Message != hookStatus.Message {
				sc.log.Infof("Hook %s %s/%s message: '%s' -> '%s'", hookStatus.Type, prev.Kind, prev.Name, prev.Message, hookStatus.Message)
			}
			sc.syncRes.Hooks[i] = &hookStatus
			return true
		}
	}
	sc.syncRes.Hooks = append(sc.syncRes.Hooks, &hookStatus)
	sc.log.Infof("Set new hook %s %s/%s. status: %s, message: %s", hookStatus.Type, hookStatus.Kind, hookStatus.Name, hookStatus.Status, hookStatus.Message)
	return true
}

// areHooksCompletedSuccessful checks if all the hooks of the specified type are completed and successful
func areHooksCompletedSuccessful(hookType appv1.HookType, hookStatuses []*appv1.HookStatus) (bool, bool) {
	isSuccessful := true
	for _, hookStatus := range hookStatuses {
		if hookStatus.Type != hookType {
			continue
		}
		if !hookStatus.Status.Completed() {
			return false, false
		}
		if !hookStatus.Status.Successful() {
			isSuccessful = false
		}
	}
	return true, isSuccessful
}

// terminate looks for any running jobs/workflow hooks and deletes the resource
func (sc *syncContext) terminate() {
	terminateSuccessful := true
	for _, hookStatus := range sc.syncRes.Hooks {
		if hookStatus.Status.Completed() {
			continue
		}
		switch hookStatus.Kind {
		case "Job", "Workflow":
			hookStatus.Status = appv1.OperationFailed
			err := sc.deleteHook(hookStatus.Name, hookStatus.Kind, hookStatus.APIVersion)
			if err != nil {
				hookStatus.Message = fmt.Sprintf("Failed to delete %s hook %s/%s: %v", hookStatus.Type, hookStatus.Kind, hookStatus.Name, err)
				terminateSuccessful = false
			} else {
				hookStatus.Message = fmt.Sprintf("Deleted %s hook %s/%s", hookStatus.Type, hookStatus.Kind, hookStatus.Name)
			}
			sc.updateHookStatus(*hookStatus)
		}
	}
	if terminateSuccessful {
		sc.setOperationPhase(appv1.OperationFailed, "Operation terminated")
	} else {
		sc.setOperationPhase(appv1.OperationError, "Operation termination had errors")
	}
}

func (sc *syncContext) deleteHook(name, kind, apiVersion string) error {
	groupVersion := strings.Split(apiVersion, "/")
	if len(groupVersion) != 2 {
		return fmt.Errorf("Failed to terminate app. Unrecognized group/version: %s", apiVersion)
	}
	gvk := schema.GroupVersionKind{
		Group:   groupVersion[0],
		Version: groupVersion[1],
		Kind:    kind,
	}
	dclient, err := sc.dynClientPool.ClientForGroupVersionKind(gvk)
	if err != nil {
		return err
	}
	apiResource, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)
	if err != nil {
		return err
	}
	resIf := dclient.Resource(apiResource, sc.namespace)
	return resIf.Delete(name, &metav1.DeleteOptions{})
}
