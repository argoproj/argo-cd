package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo/pkg/client/clientset/versioned/typed/workflow/v1alpha1"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util/kube"
)

type syncContext struct {
	comparison   *appv1.ComparisonResult
	config       *rest.Config
	namespace    string
	syncOp       *appv1.SyncOperation
	opState      *appv1.OperationState
	manifestInfo *repository.ManifestResponse
	log          *log.Entry
	// lock to protect concurrent updates of the result list
	lock sync.Mutex
}

func (s *ksonnetAppStateManager) SyncAppState(app *appv1.Application, state *appv1.OperationState) {
	// Sync requests are usually requested with ambiguous revisions (e.g. master, HEAD, v1.2.3).
	// This can change meaning when resuming operations (e.g a workflow sync). After calculating a
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
	comparison, manifestInfo, err := s.CompareAppState(app, revision, overrides)
	if err != nil {
		state.Phase = appv1.OperationError
		state.Message = err.Error()
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

	syncCtx := syncContext{
		comparison:   comparison,
		config:       clst.RESTConfig(),
		namespace:    app.Spec.Destination.Namespace,
		syncOp:       &syncOp,
		opState:      state,
		manifestInfo: manifestInfo,
		log:          log.WithFields(log.Fields{"application": app.Name}),
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

// sync has performs the actual apply or workflow based sync
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
		// NOTE: we only need to do this once per operation. The indicator we use to detect if we
		// have already performed the dry-run for this operation, is if the SyncResult resource
		// list is empty.
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
	if sc.syncOp.SyncStrategy == nil || sc.syncOp.SyncStrategy.Apply != nil {
		forceApply := false
		if sc.syncOp.SyncStrategy != nil && sc.syncOp.SyncStrategy.Apply != nil {
			forceApply = sc.syncOp.SyncStrategy.Apply.Force
		}
		sc.doApplySync(syncTasks, false, forceApply)
	} else if sc.syncOp.SyncStrategy.Workflow != nil {
		sc.doWorkflowSync(syncTasks, sc.manifestInfo.Workflows)
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

// doWorkflowSync will initiate or continue a workflow based sync. This method will be invoked when
// there are already in-flight workflows, and should be idempotent.
func (sc *syncContext) doWorkflowSync(syncTasks []syncTask, workflows []*repository.ApplicationWorkflow) {
	wfcs, err := wfclientset.NewForConfig(sc.config)
	if err != nil {
		sc.setOperationPhase(appv1.OperationError, fmt.Sprintf("Failed to initialize workflow client: %v", err))
		return
	}
	wfIf := wfcs.Workflows(sc.namespace)

	// TODO: Validate all workflows before starting the process

	for _, purpose := range []appv1.WorkflowPurpose{appv1.PurposePreSync, appv1.PurposeSync, appv1.PurposePostSync} {
		switch purpose {
		case appv1.PurposeSync:
			sc.syncNonWorkflowTasks(syncTasks)
		case appv1.PurposePostSync:
			// TODO: don't run post-sync workflows until we are in sync and progressed
		}
		err = sc.runWorkflows(wfIf, purpose, syncTasks, workflows)
		if err != nil {
			sc.setOperationPhase(appv1.OperationError, fmt.Sprintf("%s workflow error: %v", purpose, err))
			return
		}
		completed, successful := areWorkflowsCompletedSuccessful(purpose, sc.opState.Workflows)
		if !completed {
			return
		}
		if !successful {
			sc.setOperationPhase(appv1.OperationFailed, fmt.Sprintf("%s workflow failed", purpose))
			return
		}
	}
	// if we get here, all workflows successfully completed
	sc.setOperationPhase(appv1.OperationSucceeded, "successfully synced")
}

// runWorkflows iterates & filters the target manifests for resources that desire to be synced using
// a workflow. If any of them are of the specified purpose, then submit the workflow or retrieve the
// running workflow. Updates the sc.opRes.workflows with the current workflow status
func (sc *syncContext) runWorkflows(wfIf wfclientset.WorkflowInterface, purpose appv1.WorkflowPurpose, syncTasks []syncTask, workflows []*repository.ApplicationWorkflow) error {
	processedWorkflow := make(map[string]*appv1.WorkflowStatus)
	for _, task := range syncTasks {
		if task.targetObj == nil {
			continue
		}
		annotations := task.targetObj.GetAnnotations()
		if annotations == nil {
			continue
		}
		var annotationKey string
		switch purpose {
		case appv1.PurposePreSync:
			annotationKey = common.AnnotationWorkflowPreSync
		case appv1.PurposeSync:
			annotationKey = common.AnnotationWorkflowSync
		case appv1.PurposePostSync:
			annotationKey = common.AnnotationWorkflowPostSync
		default:
			continue
		}
		wfRefName := annotations[annotationKey]
		if wfRefName == "" {
			continue
		}
		wf, err := getWorkflowByRefName(wfRefName, workflows)
		if err != nil {
			return err
		}
		if wf == nil {
			return fmt.Errorf("annotation '%s' references undefined workflow: '%s'", annotationKey, wfRefName)
		}
		// use a deterministic name for the workflow so that it is idempotent
		wfName := strings.ToLower(fmt.Sprintf("%s-%s-%s-%d", purpose, wfRefName, sc.opState.SyncResult.Revision[0:7], sc.opState.StartedAt.UTC().Unix()))
		wf.Name = wfName
		wfStatus := processedWorkflow[wfName]
		if wfStatus == nil {
			sc.log.Infof("%s %s/%s via workflow %s", purpose, task.targetObj.GetKind(), task.targetObj.GetName(), wfName)
			existing, err := wfIf.Get(wfName, metav1.GetOptions{})
			if err != nil {
				if !apierr.IsNotFound(err) {
					return fmt.Errorf("Failed to get status of %s workflow '%s': %v", purpose, wfName, err)
				}
				created, err := wfIf.Create(wf)
				if err != nil {
					return fmt.Errorf("Failed to create %s workflow '%s': %v", purpose, wfName, err)
				}
				wf = created
				sc.log.Infof("Workflow '%s' submitted", wf.Name)
				sc.setOperationPhase(appv1.OperationRunning, fmt.Sprintf("Running %s workflows", purpose))
				yamlBytes, _ := yaml.Marshal(wf)
				sc.log.Debug("%s\n%s", string(yamlBytes))
			} else {
				wf = existing
			}
			wfStatus = &appv1.WorkflowStatus{
				Name:       wf.Name,
				Purpose:    purpose,
				Phase:      string(wf.Status.Phase),
				StartedAt:  wf.Status.StartedAt,
				FinishedAt: wf.Status.FinishedAt,
				Message:    wf.Status.Message,
			}
			processedWorkflow[wfName] = wfStatus
		}
		sc.setWorkflowStatus(wfStatus)

		resDetails := appv1.ResourceDetails{
			Name:      task.targetObj.GetName(),
			Kind:      task.targetObj.GetKind(),
			Namespace: sc.namespace,
			Message:   fmt.Sprintf("%s workflow '%s' %s", purpose, wfName, wfStatus.Phase),
			//Status:    ResourceSyncStatus,
		}
		sc.setResourceDetails(&resDetails)

	}
	return nil
}

// syncNonWorkflowTasks syncs or prunes the objects that are not handled by workflow.
func (sc *syncContext) syncNonWorkflowTasks(syncTasks []syncTask) {
	// We need to make sure we only do the `kubectl apply` at most once per operation.
	// Because we perform the apply before scheduling workflows, if we see any Sync or PostSync
	// workflows recorded in the list, we can safely assume we've already performed the apply.
	for _, wf := range sc.opState.Workflows {
		if wf.Purpose == appv1.PurposeSync || wf.Purpose == appv1.PurposePostSync {
			return
		}
	}
	var nonWfSyncTasks []syncTask
	for _, task := range syncTasks {
		if task.targetObj == nil {
			nonWfSyncTasks = append(nonWfSyncTasks, task)
		} else {
			annotations := task.targetObj.GetAnnotations()
			if annotations != nil {
				if annotations[common.AnnotationWorkflowSync] != "" {
					// we are doing a workflow sync and this resource is annotated with the
					// 'workflow-sync' annotation
					continue
				}
			}
			// if we get here, this resource does not have the workflow sync annotation so we
			// should perform an apply sync
			nonWfSyncTasks = append(nonWfSyncTasks, task)
		}
	}
	sc.doApplySync(nonWfSyncTasks, false, sc.syncOp.SyncStrategy.Workflow.Force)
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

func (sc *syncContext) setWorkflowStatus(wfStatus *appv1.WorkflowStatus) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	sc.log.Infof("Set workflow status: %v", wfStatus)
	for i, wfs := range sc.opState.Workflows {
		if wfs.Name == wfStatus.Name {
			sc.opState.Workflows[i] = *wfStatus
			return
		}
	}
	sc.opState.Workflows = append(sc.opState.Workflows, *wfStatus)
}

func getWorkflowByRefName(name string, workflows []*repository.ApplicationWorkflow) (*wfv1.Workflow, error) {
	for _, appWf := range workflows {
		if appWf.Name == name {
			var wf wfv1.Workflow
			err := json.Unmarshal([]byte(appWf.Manifest), &wf)
			if err != nil {
				return nil, fmt.Errorf("Failed to unmarshal workflow '%s': %v", name, err)
			}
			return &wf, nil
		}
	}
	return nil, nil
}

// areWorkflowsCompletedSuccessful checks if all the workflows of the specified purpose are completed
// and successful
func areWorkflowsCompletedSuccessful(purpose appv1.WorkflowPurpose, wfStatuses []appv1.WorkflowStatus) (bool, bool) {
	isCompleted := true
	isSuccessful := true
	for _, wfStatus := range wfStatuses {
		if wfStatus.Purpose != purpose {
			continue
		}
		phase := wfv1.NodePhase(wfStatus.Phase)
		switch phase {
		case wfv1.NodeSucceeded:
			continue
		case wfv1.NodeFailed, wfv1.NodeError:
			isSuccessful = false
		default:
			isCompleted = false
		}
	}
	return isCompleted, isSuccessful
}
