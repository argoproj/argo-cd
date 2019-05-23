package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/controller/metrics"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/health"
	"github.com/argoproj/argo-cd/util/kube"
)

type syncContext struct {
	resourceOverrides map[string]ResourceOverride
	appName           string
	proj              *AppProject
	compareResult     *comparisonResult
	config            *rest.Config
	dynamicIf         dynamic.Interface
	disco             discovery.DiscoveryInterface
	kubectl           kube.Kubectl
	namespace         string
	server            string
	syncOp            *SyncOperation
	syncRes           *SyncOperationResult
	syncResources     []SyncOperationResource
	opState           *OperationState
	log               *log.Entry
	// lock to protect concurrent updates of the result list
	lock sync.Mutex
}

func (m *appStateManager) SyncAppState(app *Application, state *OperationState) {
	// Sync requests might be requested with ambiguous revisions (e.g. master, HEAD, v1.2.3).
	// This can change meaning when resuming operations (e.g a hook sync). After calculating a
	// concrete git commit SHA, the SHA is remembered in the status.operationState.syncResult field.
	// This ensures that when resuming an operation, we sync to the same revision that we initially
	// started with.
	var revision string
	var syncOp SyncOperation
	var syncRes *SyncOperationResult
	var syncResources []SyncOperationResource
	var source ApplicationSource

	if state.Operation.Sync == nil {
		state.Phase = OperationFailed
		state.Message = "Invalid operation request: no operation specified"
		return
	}
	syncOp = *state.Operation.Sync
	if syncOp.Source == nil {
		// normal sync case (where source is taken from app.spec.source)
		source = app.Spec.Source
	} else {
		// rollback case
		source = *state.Operation.Sync.Source
	}
	syncResources = syncOp.Resources
	if state.SyncResult != nil {
		syncRes = state.SyncResult
		revision = state.SyncResult.Revision
	} else {
		syncRes = &SyncOperationResult{}
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

	compareResult, err := m.CompareAppState(app, revision, source, false)
	if err != nil {
		state.Phase = OperationError
		state.Message = err.Error()
		return
	}

	// If there are any error conditions, do not perform the operation
	errConditions := make([]ApplicationCondition, 0)
	for i := range compareResult.conditions {
		if compareResult.conditions[i].IsError() {
			errConditions = append(errConditions, compareResult.conditions[i])
		}
	}
	if len(errConditions) > 0 {
		state.Phase = OperationError
		state.Message = argo.FormatAppConditions(errConditions)
		return
	}

	// We now have a concrete commit SHA. Save this in the sync result revision so that we remember
	// what we should be syncing to when resuming operations.
	syncRes.Revision = compareResult.syncStatus.Revision

	clst, err := m.db.GetCluster(context.Background(), app.Spec.Destination.Server)
	if err != nil {
		state.Phase = OperationError
		state.Message = err.Error()
		return
	}

	restConfig := metrics.AddMetricsTransportWrapper(m.metricsServer, app, clst.RESTConfig())
	dynamicIf, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		state.Phase = OperationError
		state.Message = fmt.Sprintf("Failed to initialize dynamic client: %v", err)
		return
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		state.Phase = OperationError
		state.Message = fmt.Sprintf("Failed to initialize discovery client: %v", err)
		return
	}

	proj, err := argo.GetAppProject(&app.Spec, v1alpha1.NewAppProjectLister(m.projInformer.GetIndexer()), m.namespace)
	if err != nil {
		state.Phase = OperationError
		state.Message = fmt.Sprintf("Failed to load application project: %v", err)
		return
	}

	syncCtx := syncContext{
		resourceOverrides: m.settings.ResourceOverrides,
		appName:           app.Name,
		proj:              proj,
		compareResult:     compareResult,
		config:            restConfig,
		dynamicIf:         dynamicIf,
		disco:             disco,
		kubectl:           m.kubectl,
		namespace:         app.Spec.Destination.Namespace,
		server:            app.Spec.Destination.Server,
		syncOp:            &syncOp,
		syncRes:           syncRes,
		syncResources:     syncResources,
		opState:           state,
		log:               log.WithFields(log.Fields{"application": app.Name}),
	}

	if state.Phase == OperationTerminating {
		syncCtx.terminate()
	} else {
		syncCtx.sync()
	}

	syncCtx.log.Info("sync/terminate complete")

	if !syncOp.DryRun && !syncCtx.isSelectiveSync() && syncCtx.opState.Phase.Successful() {
		err := m.persistRevisionHistory(app, compareResult.syncStatus.Revision, source)
		if err != nil {
			syncCtx.setOperationPhase(OperationError, fmt.Sprintf("failed to record sync to history: %v", err))
		}
	}
}

func (sc *syncContext) getHealthStatus(obj *unstructured.Unstructured) (healthStatus HealthStatusCode, message string) {
	resourceHealth, err := health.GetResourceHealth(obj, sc.resourceOverrides)
	if err != nil {
		return HealthStatusUnknown, err.Error()
	}
	if resourceHealth == nil {
		return HealthStatusMissing, "missing"
	}
	return resourceHealth.Status, resourceHealth.Message
}

// sync has performs the actual apply or hook based syncStatus
func (sc *syncContext) sync() {
	sc.log.Info("syncing")
	tasks, successful := sc.getSyncTasks()
	if !successful {
		sc.setOperationPhase(OperationFailed, "one or more synchronization tasks are not valid")
		return
	}

	sc.log.WithFields(log.Fields{"tasks": tasks}).Debug("tasks")

	// Perform a `kubectl apply --dry-run` against all the manifests. This will detect most (but
	// not all) validation issues with the user's manifests (e.g. will detect syntax issues, but
	// will not not detect if they are mutating immutable fields). If anything fails, we will refuse
	// to perform the sync. we only wish to do this once per operation, performing additional dry-runs
	// is harmless, but redundant. The indicator we use to detect if we have already performed
	// the dry-run for this operation, is if the resource or hook list is empty.
	if sc.notStarted() {
		sc.log.Info("dry-run")
		if !sc.runTasks(tasks, true) {
			sc.setOperationPhase(OperationFailed, "one or more objects failed to apply (dry run)")
			return
		}
	}

	// update status of any tasks that are running, note that this must exclude pruning tasks
	for _, task := range tasks.Filter(func(t *syncTask) bool {
		return t.running()
	}) {
		healthStatus, message := sc.getHealthStatus(task.liveObj)
		log.WithFields(log.Fields{"task": task, "healthStatus": healthStatus, "message": message}).Debug("attempting to update health of running task")
		if task.isHook() {
			// update the hook's result
			operationState, message := getOperationPhase(task.liveObj)
			sc.setResourceResult(task, "", operationState, message)

			// maybe delete the hook
			if enforceHookDeletePolicy(task.liveObj, task.operationState) {
				err := sc.deleteResource(task)
				if err != nil {
					sc.setResourceResult(task, "", OperationError, fmt.Sprintf("failed to delete resource: %v", err))
				}
			}
		} else {
			// this must be calculated on the live object
			// TODO - what about resources without health? e.g. secret
			switch healthStatus {
			// TODO are we correct here?
			case HealthStatusHealthy:
				sc.setResourceResult(task, task.syncStatus, OperationSucceeded, message)
			case HealthStatusDegraded:
				sc.setResourceResult(task, task.syncStatus, OperationFailed, message)
			}
		}
	}

	// any running tasks, lets wait...
	if tasks.Find(func(t *syncTask) bool { return t.running() }) != nil {
		sc.setOperationPhase(OperationRunning, "one or more tasks are running")
		return
	}

	// any completed by unsuccessful tasks is a total failure.
	if tasks.Find(func(t *syncTask) bool { return t.completed() && !t.successful() }) != nil {
		sc.setOperationPhase(OperationFailed, "one or more synchronization tasks completed unsuccessfully")
		return
	}

	sc.log.WithFields(log.Fields{"tasks": tasks}).Debug("filtering out completed tasks")
	// remove tasks that are completed, note  we assume that there are no running tasks
	tasks = tasks.Filter(func(t *syncTask) bool { return !t.completed() })

	// If no sync tasks were generated (e.g., in case all application manifests have been removed),
	// set the sync operation as successful.
	if len(tasks) == 0 {
		sc.setOperationPhase(OperationSucceeded, "successfully synced")
		return
	}

	// remove any tasks not in this wave
	phase := tasks.phase()
	wave := tasks.wave()

	sc.log.WithFields(log.Fields{"phase": phase, "wave": wave, "tasks": tasks}).Debug("filtering tasks in correct phase and wave")
	tasks = tasks.Filter(func(t *syncTask) bool { return t.phase == phase && t.wave() == wave })

	sc.setOperationPhase(OperationRunning, fmt.Sprintf("running phase='%s' wave=%d", phase, wave))

	sc.log.WithFields(log.Fields{"tasks": tasks}).Info("wet-run")
	if !sc.runTasks(tasks, false) {
		sc.setOperationPhase(OperationFailed, "one or more objects failed to apply")
	}
}

func (sc *syncContext) notStarted() bool {
	return len(sc.syncRes.Resources) == 0
}

func (sc *syncContext) isSelectiveSync() bool {
	return sc.syncResources != nil
}

// this essentially enforces the old "apply" behaviour
func (sc *syncContext) skipHooks() bool {
	// All objects passed a `kubectl apply --dry-run`, so we are now ready to actually perform the sync.
	// default sync strategy to hook if no strategy
	// TODO - can we get rid of apply based strategy?
	return sc.syncOp.IsApplyStrategy() || sc.isSelectiveSync()
}

func (sc *syncContext) containsResource(resourceState managedResource) bool {
	return !sc.isSelectiveSync() ||
		(resourceState.Live != nil && argo.ContainsSyncResource(resourceState.Live.GetName(), resourceState.Live.GroupVersionKind(), sc.syncResources)) ||
		(resourceState.Target != nil && argo.ContainsSyncResource(resourceState.Target.GetName(), resourceState.Target.GroupVersionKind(), sc.syncResources))
}

// generateSyncTasks() generates the list of sync tasks we will be performing during this syncStatus.
func (sc *syncContext) getSyncTasks() (tasks syncTasks, successful bool) {
	tasks = syncTasks{}
	successful = true

	for _, resource := range sc.compareResult.managedResources {
		if !sc.containsResource(resource) {
			log.WithFields(log.Fields{"resourceGroup": resource.Group, "resourceKind": resource.Kind, "resourceName": resource.Name}).Debug("skipping")
			continue
		}
		obj := resource.Target
		if obj == nil {
			obj = resource.Live
		}

		if sc.skipHooks() && isHook(obj) {
			continue
		}

		for _, phase := range syncPhases(obj) {
			tasks = append(tasks, &syncTask{
				phase:     phase,
				liveObj:   resource.Live,
				targetObj: resource.Target,
			})
		}
	}

	sc.log.WithFields(log.Fields{"tasks": tasks}).Debug("tasks")

	for _, task := range tasks {
		if task.targetObj == nil {
			continue
		}

		// Hook resources names are deterministic, whether they are defined by the user (metadata.name),
		// or formulated at the time of the operation (metadata.generateName). If user specifies
		// metadata.generateName, then we will generate a formulated metadata.name before submission.

		// TODO - test (probably a bug here)
		if task.targetObj.GetName() == "" {
			postfix := strings.ToLower(fmt.Sprintf("%s-%s-%d", sc.syncRes.Revision[0:7], task.phase, sc.opState.StartedAt.UTC().Unix()))
			generateName := task.targetObj.GetGenerateName()
			task.targetObj.SetName(fmt.Sprintf("%s%s", generateName, postfix))
		}

		// TODO - test
		if task.targetObj.GetNamespace() == "" {
			// If target object's namespace is empty, we set namespace in the object. We do
			// this even though it might be a cluster-scoped resource. This prevents any
			// possibility of the resource from unintentionally becoming created in the
			// namespace during the `kubectl apply`
			task.targetObj.SetNamespace(sc.namespace)
		}
	}

	for _, task := range tasks {
		// TODO - no version?
		_, result := sc.syncRes.Resources.Find(task.group(), task.kind(), task.namespace(), task.name(), task.phase)
		if result != nil {
			task.syncStatus = result.Status
			task.operationState = result.HookPhase
			task.message = result.Message
		}
	}

	for _, task := range tasks {
		serverRes, err := kube.ServerResourceForGroupVersionKind(sc.disco, task.groupVersionKind())
		if err != nil {
			// Special case for custom resources: if CRD is not yet known by the K8s API server,
			// skip verification during `kubectl apply --dry-run` since we expect the CRD
			// to be created during app synchronization.
			if apierr.IsNotFound(err) && sc.hasCRDOfGroupKind(task.group(), task.kind()) {
				sc.log.WithFields(log.Fields{"task": task}).Info("skip dry-run for custom resource")
				task.skipDryRun = true
			} else {
				sc.setResourceResult(task, ResultCodeSyncFailed, "", err.Error())
				successful = false
			}
		} else {
			if !sc.proj.IsResourcePermitted(metav1.GroupKind{Group: task.group(), Kind: task.kind()}, serverRes.Namespaced) {
				sc.setResourceResult(task, ResultCodeSyncFailed, "", fmt.Sprintf("Resource %s:%s is not permitted in project %s.", task.group(), task.kind(), sc.proj.Name))
				successful = false
			}
			if serverRes.Namespaced && !sc.proj.IsDestinationPermitted(ApplicationDestination{Namespace: task.namespace(), Server: sc.server}) {
				sc.setResourceResult(task, ResultCodeSyncFailed, "", fmt.Sprintf("namespace %v is not permitted in project '%s'", task.namespace(), sc.proj.Name))
				successful = false
			}
		}
	}

	sort.Sort(tasks)

	return tasks, successful
}

func (sc *syncContext) setOperationPhase(phase OperationPhase, message string) {
	if sc.opState.Phase != phase || sc.opState.Message != message {
		sc.log.Infof("Updating operation state. phase: %s -> %s, message: '%s' -> '%s'", sc.opState.Phase, phase, sc.opState.Message, message)
	}
	sc.opState.Phase = phase
	sc.opState.Message = message
}

// applyObject performs a `kubectl apply` of a single resource
func (sc *syncContext) applyObject(targetObj *unstructured.Unstructured, dryRun bool, force bool) (ResultCode, string) {
	message, err := sc.kubectl.ApplyResource(sc.config, targetObj, targetObj.GetNamespace(), dryRun, force)
	if err != nil {
		return ResultCodeSyncFailed, err.Error()
	}
	return ResultCodeSynced, message
}

// pruneObject deletes the object if both prune is true and dryRun is false. Otherwise appropriate message
func (sc *syncContext) pruneObject(liveObj *unstructured.Unstructured, prune, dryRun bool) (ResultCode, string) {
	if prune {
		if dryRun {
			return ResultCodePruned, "pruned (dry run)"
		} else {
			// Skip deletion if object is already marked for deletion, so we don't cause a resource update hotloop
			deletionTimestamp := liveObj.GetDeletionTimestamp()
			if deletionTimestamp == nil || deletionTimestamp.IsZero() {
				err := sc.kubectl.DeleteResource(sc.config, liveObj.GroupVersionKind(), liveObj.GetName(), liveObj.GetNamespace(), false)
				if err != nil {
					return ResultCodeSyncFailed, err.Error()
				}
			}
			return ResultCodePruned, "pruned"
		}
	} else {
		return ResultCodePruneSkipped, "ignored (requires pruning)"
	}
}

func (sc *syncContext) hasCRDOfGroupKind(group string, kind string) bool {
	for _, res := range sc.compareResult.managedResources {
		if res.Target != nil && kube.IsCRD(res.Target) {
			crdGroup, ok, err := unstructured.NestedString(res.Target.Object, "spec", "group")
			if err != nil || !ok {
				continue
			}
			crdKind, ok, err := unstructured.NestedString(res.Target.Object, "spec", "names", "kind")
			if err != nil || !ok {
				continue
			}
			if group == crdGroup && crdKind == kind {
				return true
			}
		}
	}
	return false
}

// terminate looks for any running jobs/workflow hooks and deletes the resource
func (sc *syncContext) terminate() {
	terminateSuccessful := true
	sc.log.Info("terminating")
	tasks, _ := sc.getSyncTasks()
	for _, task := range tasks {
		if !task.isHook() || !task.completed() {
			continue
		}
		if isRunnable(task.groupVersionKind()) {
			err := sc.deleteResource(task)
			if err != nil {
				sc.setResourceResult(task, "", OperationFailed, fmt.Sprintf("Failed to delete: %v", err))
				terminateSuccessful = false
			} else {
				sc.setResourceResult(task, "", OperationSucceeded, fmt.Sprintf("Deleted"))
			}
		}
	}
	if terminateSuccessful {
		sc.setOperationPhase(OperationFailed, "Operation terminated")
	} else {
		sc.setOperationPhase(OperationError, "Operation termination had errors")
	}
}

func (sc *syncContext) deleteResource(task *syncTask) error {
	sc.log.WithFields(log.Fields{"task": task}).Info("deleting task")
	apiResource, err := kube.ServerResourceForGroupVersionKind(sc.disco, task.groupVersionKind())
	if err != nil {
		return err
	}
	resource := kube.ToGroupVersionResource(task.groupVersionKind().GroupVersion().String(), apiResource)
	resIf := kube.ToResourceInterface(sc.dynamicIf, apiResource, resource, task.namespace())
	propagationPolicy := metav1.DeletePropagationForeground
	return resIf.Delete(task.name(), &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}

var operationPhases = map[ResultCode]OperationPhase{
	ResultCodeSynced:       OperationRunning,
	ResultCodeSyncFailed:   OperationFailed,
	ResultCodePruned:       OperationSucceeded,
	ResultCodePruneSkipped: OperationSucceeded,
}

func (sc *syncContext) runTasks(tasks syncTasks, dryRun bool) (successful bool) {

	dryRun = dryRun || sc.syncOp.DryRun

	sc.log.WithFields(log.Fields{"numTasks": len(tasks), "dryRun": dryRun}).Info("running tasks")

	successful = true
	var createTasks syncTasks
	var pruneTasks syncTasks

	for _, task := range tasks {
		if task.isPrune() {
			pruneTasks = append(pruneTasks, task)
		} else {
			createTasks = append(createTasks, task)
		}
	}

	var wg sync.WaitGroup
	for _, task := range pruneTasks {
		wg.Add(1)
		go func(t *syncTask) {
			defer wg.Done()
			sc.log.WithFields(log.Fields{"dryRun": dryRun, "task": t}).Info("pruning")
			result, message := sc.pruneObject(t.liveObj, sc.syncOp.Prune, dryRun)
			if result == ResultCodeSyncFailed {
				successful = false
			}
			if !dryRun || result == ResultCodeSyncFailed {
				sc.setResourceResult(t, result, operationPhases[result], message)
			}
		}(task)
	}
	wg.Wait()

	processCreateTasks := func(tasks syncTasks) {
		var createWg sync.WaitGroup
		for _, task := range tasks {
			if dryRun && task.skipDryRun {
				continue
			}
			createWg.Add(1)
			go func(t *syncTask) {
				defer createWg.Done()
				sc.log.WithFields(log.Fields{"dryRun": dryRun, "task": t}).Info("applying")
				result, message := sc.applyObject(t.targetObj, dryRun, sc.syncOp.SyncStrategy.Force())
				if result == ResultCodeSyncFailed {
					successful = false
				}
				if !dryRun || result == ResultCodeSyncFailed {
					sc.setResourceResult(t, result, operationPhases[result], message)
				}
			}(task)
		}
		createWg.Wait()
	}

	var tasksGroup syncTasks
	for _, task := range createTasks {
		//Only wait if the type of the next task is different than the previous type
		if len(tasksGroup) > 0 && tasksGroup[0].targetObj.GetKind() != task.kind() {
			processCreateTasks(tasksGroup)
			tasksGroup = syncTasks{task}
		} else {
			tasksGroup = append(tasksGroup, task)
		}
	}
	if len(tasksGroup) > 0 {
		processCreateTasks(tasksGroup)
	}
	return successful
}

// setResourceResult sets a resource details in the SyncResult.Resources list
func (sc *syncContext) setResourceResult(task *syncTask, syncStatus ResultCode, operationState OperationPhase, message string) {

	task.syncStatus = syncStatus
	task.operationState = operationState
	task.message = message

	sc.lock.Lock()
	defer sc.lock.Unlock()
	i, existing := sc.syncRes.Resources.Find(task.group(), task.kind(), task.namespace(), task.name(), task.phase)

	res := ResourceResult{
		Group:     task.group(),
		Version:   task.version(),
		Kind:      task.kind(),
		Namespace: task.namespace(),
		Name:      task.name(),
		Status:    task.syncStatus,
		Message:   task.message,
		HookPhase: task.operationState,
		SyncPhase: task.phase,
	}

	logCtx := sc.log.WithFields(log.Fields{"namespace": task.namespace(), "kind": task.kind(), "name": task.name(), "phase": task.phase})

	if existing != nil {
		// update existing value
		if res.Status != existing.Status || res.HookPhase != existing.HookPhase || res.Message != existing.Message {
			logCtx.Infof("updating resource result, status: '%s' -> '%s', phase '%s' -> '%s', message '%s' -> '%s'",
				existing.Status, res.Status,
				existing.HookPhase, res.HookPhase,
				existing.Message, res.Message)
		}
		sc.syncRes.Resources[i] = res
	} else {
		logCtx.Infof("adding resource result, status: '%s', phase: '%s', message: '%s'", res.Status, res.HookPhase, res.Message)
		sc.syncRes.Resources = append(sc.syncRes.Resources, res)
	}
}
