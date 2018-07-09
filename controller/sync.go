package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	comparison    *appv1.ComparisonResult
	config        *rest.Config
	dynClientPool dynamic.ClientPool
	disco         *discovery.DiscoveryClient
	namespace     string
	syncOp        *appv1.SyncOperation
	opState       *appv1.OperationState
	manifestInfo  *repository.ManifestResponse
	log           *log.Entry
	// lock to protect concurrent updates of the result list
	lock sync.Mutex
}

func (s *ksonnetAppStateManager) SyncAppState(app *appv1.Application, state *appv1.OperationState) {
	// Sync requests are usually requested with ambiguous revisions (e.g. master, HEAD, v1.2.3).
	// This can change meaning when resuming operations (e.g a hook sync). After calculating a
	// concrete git commit SHA, the SHA remembered in the status.operationState.syncResult and
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
			Revision: deploymentInfo.Revision,
			DryRun:   state.Operation.Rollback.DryRun,
			Prune:    state.Operation.Rollback.Prune,
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
	comparison, manifestInfo, errConditions, err := s.CompareAppState(app, revision, overrides)
	if err != nil {
		state.Phase = appv1.OperationError
		state.Message = err.Error()
		return
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
		comparison:    comparison,
		config:        restConfig,
		dynClientPool: dynClientPool,
		disco:         disco,
		namespace:     app.Spec.Destination.Namespace,
		syncOp:        &syncOp,
		opState:       state,
		manifestInfo:  manifestInfo,
		log:           log.WithFields(log.Fields{"application": app.Name}),
	}

	syncCtx.sync()
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
	if len(sc.opState.SyncResult.Resources) == 0 {
		// Optimization: we only wish to do this once per operation, performing additional dry-runs
		// is harmless, but redundant. The indicator we use to detect if we have already performed
		// the dry-run for this operation, is if the SyncResult resource list is empty.
		successful = sc.doApplySync(syncTasks, true, false)
		if !successful {
			return
		}
	}
	if sc.syncOp.DryRun {
		sc.setOperationPhase(appv1.OperationSucceeded, "successfully synced (dry run)")
		return
	}

	// All objects passed a `kubectl apply --dry-run`, so we are now ready to actually perform the sync.
	if sc.syncOp.SyncStrategy.Apply != nil {
		forceApply := false
		if sc.syncOp.SyncStrategy != nil && sc.syncOp.SyncStrategy.Apply != nil {
			forceApply = sc.syncOp.SyncStrategy.Apply.Force
		}
		sc.doApplySync(syncTasks, false, forceApply)
	} else if sc.syncOp.SyncStrategy.Hook != nil || sc.syncOp.SyncStrategy == nil {
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

func (sc *syncContext) setOperationPhase(phase appv1.OperationPhase, message string) {
	sc.opState.Phase = phase
	sc.opState.Message = message
}

// applyObject performs a `kubectl apply` of a single resource
func (sc *syncContext) applyObject(targetObj *unstructured.Unstructured, dryRun bool, force bool) (appv1.ResourceDetails, bool) {
	resDetails := appv1.ResourceDetails{
		Name:      targetObj.GetName(),
		Kind:      targetObj.GetKind(),
		Namespace: sc.namespace,
	}
	message, err := kube.ApplyResource(sc.config, targetObj, sc.namespace, dryRun, force)
	if err != nil {
		resDetails.Message = err.Error()
		resDetails.Status = appv1.ResourceDetailsSyncFailed
		return resDetails, false
	}
	resDetails.Message = message
	resDetails.Status = appv1.ResourceDetailsSynced
	return resDetails, true
}

// pruneObject deletes the object if both prune is true and dryRun is false. Otherwise appropriate message
func (sc *syncContext) pruneObject(liveObj *unstructured.Unstructured, prune, dryRun bool) (appv1.ResourceDetails, bool) {
	resDetails := appv1.ResourceDetails{
		Name:      liveObj.GetName(),
		Kind:      liveObj.GetKind(),
		Namespace: liveObj.GetNamespace(),
	}
	successful := true
	if prune {
		if dryRun {
			resDetails.Message = "pruned (dry run)"
			resDetails.Status = appv1.ResourceDetailsSyncedAndPruned
		} else {
			err := kube.DeleteResource(sc.config, liveObj, sc.namespace)
			if err != nil {
				resDetails.Message = err.Error()
				resDetails.Status = appv1.ResourceDetailsSyncFailed
				successful = false
			} else {
				resDetails.Message = "pruned"
				resDetails.Status = appv1.ResourceDetailsSyncedAndPruned
			}
		}
	} else {
		resDetails.Message = "ignored (requires pruning)"
		resDetails.Status = appv1.ResourceDetailsPruningRequired
	}
	return resDetails, successful
}

// performs an apply of the given sync tasks.
func (sc *syncContext) doApplySync(syncTasks []syncTask, dryRun, force bool) bool {
	syncSuccessful := true
	// apply all resources in parallel
	var wg sync.WaitGroup
	for _, task := range syncTasks {
		defer func(t syncTask) {
			wg.Add(1)
			defer wg.Done()
			var resDetails appv1.ResourceDetails
			var successful bool
			if t.targetObj == nil {
				resDetails, successful = sc.pruneObject(t.liveObj, sc.syncOp.Prune, dryRun)
			} else {
				if !isSyncedResource(t.targetObj) {
					return
				}
				resDetails, successful = sc.applyObject(t.targetObj, dryRun, force)
			}
			if !successful {
				syncSuccessful = false
			}
			sc.setResourceDetails(&resDetails)
		}(task)
	}
	wg.Wait()
	if !syncSuccessful {
		errMsg := "one or more objects failed to apply"
		if dryRun {
			errMsg += " (dry run)"
		}
		sc.setOperationPhase(appv1.OperationFailed, errMsg)
		return false
	}
	if !dryRun {
		sc.setOperationPhase(appv1.OperationSucceeded, "successfully synced")
	}
	return true
}

// doHookSync initiates or continues a hook-based sync. This method will be invoked when there may
// already be in-flight jobs/workflows, and should be idempotent.
func (sc *syncContext) doHookSync(syncTasks []syncTask, hooks []*unstructured.Unstructured) {
	for _, hookType := range []appv1.HookType{appv1.HookTypePreSync, appv1.HookTypeSync, appv1.HookTypePostSync} {
		switch hookType {
		case appv1.HookTypeSync:
			// before performing the sync via hook, apply any manifests which are missing the hook annotation
			sc.syncNonHookTasks(syncTasks)
		case appv1.HookTypePostSync:
			// TODO: don't run post-sync workflows until we are in sync and progressed
		}
		err := sc.runHooks(hooks, hookType)
		if err != nil {
			sc.setOperationPhase(appv1.OperationError, fmt.Sprintf("%s hook error: %v", hookType, err))
			return
		}
		completed, successful := areHooksCompletedSuccessful(hookType, sc.opState.HookResources)
		if !completed {
			return
		}
		if !successful {
			sc.setOperationPhase(appv1.OperationFailed, fmt.Sprintf("%s hook failed", hookType))
			return
		}
	}
	// if we get here, all hooks successfully completed
	sc.setOperationPhase(appv1.OperationSucceeded, "successfully synced")
}

func (sc *syncContext) getHooks() ([]*unstructured.Unstructured, error) {
	var hooks []*unstructured.Unstructured
	for _, manifest := range sc.manifestInfo.Manifests {
		var hook unstructured.Unstructured
		err := json.Unmarshal([]byte(manifest), &hook)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, &hook)
	}
	return hooks, nil
}

// runHooks iterates & filters the target manifests for resources of the specified hook type, then
// creates the resource. Updates the sc.opRes.hooks with the current status
func (sc *syncContext) runHooks(hooks []*unstructured.Unstructured, hookType appv1.HookType) error {
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
		err := sc.runHook(hook, hookType)
		if err != nil {
			return err
		}
	}
	return nil
}

func (sc *syncContext) runHook(hook *unstructured.Unstructured, hookType appv1.HookType) error {
	// Hook resources names are deterministic, whether they are defined by the user (metadata.name),
	// or formulated at the time of the operation (metadata.generateName). If user specifies
	// metadata.generateName, then we will generate a formulated metadata.name before submission.
	if hook.GetName() == "" {
		postfix := strings.ToLower(fmt.Sprintf("%s-%s-%d", hookType, sc.opState.SyncResult.Revision[0:7], sc.opState.StartedAt.UTC().Unix()))
		generatedName := hook.GetGenerateName()
		hook = hook.DeepCopy()
		hook.SetName(fmt.Sprintf("%s%s", generatedName, postfix))
	}
	// Check our hook statuses to see if we already completed this hook.
	// If so, this method is a noop
	prevStatus := sc.getHookStatus(hook, hookType)
	if prevStatus != nil && prevStatus.Status.Completed() {
		return nil
	}

	gvk := hook.GroupVersionKind()
	dclient, err := sc.dynClientPool.ClientForGroupVersionKind(gvk)
	if err != nil {
		return err
	}
	apiResource, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)
	if err != nil {
		return err
	}
	resIf := dclient.Resource(apiResource, sc.namespace)

	var liveObj *unstructured.Unstructured
	existing, err := resIf.Get(hook.GetName(), metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			return fmt.Errorf("Failed to get status of %s hook %s '%s': %v", hookType, gvk, hook.GetName(), err)
		}
		_, err := kube.ApplyResource(sc.config, hook, sc.namespace, false, false)
		if err != nil {
			return fmt.Errorf("Failed to create %s hook %s '%s': %v", hookType, gvk, hook.GetName(), err)
		}
		created, err := resIf.Get(hook.GetName(), metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("Failed to get status of %s hook %s '%s': %v", hookType, gvk, hook.GetName(), err)
		}
		sc.log.Infof("%s hook %s '%s' created", hookType, gvk, created.GetName())
		sc.setOperationPhase(appv1.OperationRunning, fmt.Sprintf("Running %s hooks", hookType))
		liveObj = created
	} else {
		liveObj = existing
	}
	sc.setHookStatus(liveObj, hookType)
	return nil
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

// isSyncedResource tells whether or not the supplied object is a normal, synced resource,
// and not a lifecycle hook.
func isSyncedResource(obj *unstructured.Unstructured) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return true
	}
	resHookTypes := strings.Split(annotations[common.AnnotationHook], ",")
	for _, hookType := range resHookTypes {
		hookType = strings.TrimSpace(hookType)
		switch appv1.HookType(hookType) {
		case appv1.HookTypePreSync, appv1.HookTypeSync, appv1.HookTypePostSync:
			return false
		}
	}
	return true
}

// syncNonHookTasks syncs or prunes the objects that are not handled by hooks
func (sc *syncContext) syncNonHookTasks(syncTasks []syncTask) {
	// We need to make sure we only do the `kubectl apply` at most once per operation.
	// Because we perform the apply before immediately before creating sync hooks, if we see any
	// Sync or PostSync hooks recorded in the list, we can safely assume we've already performed
	// the apply.
	for _, hr := range sc.opState.HookResources {
		if hr.Type == appv1.HookTypeSync || hr.Type == appv1.HookTypePostSync {
			return
		}
	}
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
	sc.doApplySync(nonHookTasks, false, sc.syncOp.SyncStrategy.Hook.Force)
}

// setResourceDetails sets a resource details in the SyncResult.Resources list
func (sc *syncContext) setResourceDetails(details *appv1.ResourceDetails) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	sc.log.Infof("Set sync result: %v", details)
	for i, res := range sc.opState.SyncResult.Resources {
		if res.Kind == details.Kind && res.Name == details.Name {
			// update existing value
			sc.opState.SyncResult.Resources[i] = details
			return
		}
	}
	sc.opState.SyncResult.Resources = append(sc.opState.SyncResult.Resources, details)
}

func (sc *syncContext) getHookStatus(hookObj *unstructured.Unstructured, hookType appv1.HookType) *appv1.HookStatus {
	for _, hr := range sc.opState.HookResources {
		if hr.Name == hookObj.GetName() && hr.Kind == hookObj.GetKind() && hr.Type == hookType {
			return &hr
		}
	}
	return nil
}

func (sc *syncContext) setHookStatus(hook *unstructured.Unstructured, hookType appv1.HookType) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	// TODO: how to handle situation when resource is part of multiple hooks?
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

	sc.log.Infof("Set hook status: %v", hookStatus)
	for i, hr := range sc.opState.HookResources {
		if hr.Name == hookStatus.Name && hr.Kind == hookStatus.Kind && hr.Type == hookType {
			sc.opState.HookResources[i] = hookStatus
			return
		}
	}
	sc.opState.HookResources = append(sc.opState.HookResources, hookStatus)
}

// areHooksCompletedSuccessful checks if all the hooks of the specified type are completed and successful
func areHooksCompletedSuccessful(hookType appv1.HookType, hookStatuses []appv1.HookStatus) (bool, bool) {
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
