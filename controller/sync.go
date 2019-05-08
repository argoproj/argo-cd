package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"

	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/controller/metrics"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/health"
	"github.com/argoproj/argo-cd/util/hook"
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
	// concrete git commit SHA, the SHA is remembered in the status.operationState.result field.
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
		// status.operationState.result.source. must be set properly since auto-sync relies
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

	if !syncOp.DryRun && !syncOp.IsSelectiveSync() && syncCtx.opState.Phase.Successful() {
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
		return HealthStatusMissing, ""
	}
	return resourceHealth.Status, resourceHealth.Message
}

// sync has performs the actual apply or hook based sync
func (sc *syncContext) sync() {
	sc.log.Info("syncing")
	tasks, successful := sc.getSyncTasks()
	if !successful {
		sc.setOperationPhase(OperationFailed, "one or more synchronization tasks are not valid")
		return
	}

	// Perform a `kubectl apply --dry-run` against all the manifests. This will detect most (but
	// not all) validation issues with the user's manifests (e.g. will detect syntax issues, but
	// will not not detect if they are mutating immutable fields). If anything fails, we will refuse
	// to perform the sync.
	if sc.notStarted() {
		sc.log.Info("dry-run")
		// Optimization: we only wish to do this once per operation, performing additional dry-runs
		// is harmless, but redundant. The indicator we use to detect if we have already performed
		// the dry-run for this operation, is if the resource or hook list is empty.
		if !sc.runTasks(tasks, true) {
			sc.setOperationPhase(OperationFailed, "one or more objects failed to apply (dry run)")
			return
		}
	}

	// update status of any tasks that have already run, then clean-up
	for _, task := range tasks.Filter(func(t syncTask) bool {
		return t.result.running()
	}) {
		if task.isHook() {
			// update the hook's result
			operation, message := getOperationPhase(task.getObj())
			sc.setResourceResult(&task, result{operation: operation, message: message})

			// maybe delete the hook
			if enforceHookDeletePolicy(task.getObj(), task.result.operation) {
				err := sc.deleteHook(task.getName(), task.getNamespace(), task.groupVersionKind())
				if err != nil {
					sc.setResourceResult(&task, result{operation: OperationError, message: fmt.Sprintf("failed to delete hook: %v", err)})
				}
			}
		} else if !task.isPrune() {
			healthStatus, message := sc.getHealthStatus(task.getObj())
			switch healthStatus {
			case HealthStatusHealthy:
				sc.setResourceResult(&task, result{sync: task.result.sync, operation: OperationSucceeded, message: message})
			case HealthStatusDegraded:
				sc.setResourceResult(&task, result{sync: task.result.sync, operation: OperationFailed, message: message})
			}
		}
	}

	sc.log.WithFields(log.Fields{"numTasks": tasks.Len()}).Info("tasks pre-filtering")

	// remove complete operations, note
	tasks = tasks.Filter(func(task syncTask) bool {
		return task.result.operation == ""
	})

	sc.log.WithFields(log.Fields{"numTasks": tasks.Len()}).Info("tasks mid-filtering")

	// remove any tasks not in this wave
	if len(tasks) > 0 {
		phase := tasks[0].syncPhase
		wave := tasks[0].getWave()
		sc.log.WithFields(log.Fields{"phase": phase, "wave": wave, "numTasks": len(tasks)}).Info("filtering tasks in correct phase and wave")
		tasks = tasks.Filter(func(task syncTask) bool {
			return task.syncPhase == phase && task.getWave() == wave
		})
		if len(tasks) == 0 {
			panic("this can never happen")
		}
	}

	sc.log.WithFields(log.Fields{"numTasks": tasks.Len()}).Info("tasks post-filtering")

	// If no sync tasks were generated (e.g., in case all application manifests have been removed),
	// set the sync operation as successful.
	if len(tasks) == 0 {
		sc.setOperationPhase(OperationSucceeded, "successfully synced")
		return
	}

	sc.log.WithFields(log.Fields{"numTasks": len(tasks)}).Info("wet run")

	if !sc.runTasks(tasks, false) {
		sc.setOperationPhase(OperationFailed, "one or more objects failed to apply")
	} else {
		sc.setOperationPhase(OperationRunning, "running")
	}
}

func (sc *syncContext) notStarted() bool {
	return len(sc.syncRes.Resources) == 0
}

func (sc *syncContext) skipHooks() bool {
	// All objects passed a `kubectl apply --dry-run`, so we are now ready to actually perform the sync.
	// default sync strategy to hook if no strategy
	return sc.syncOp.SyncStrategy != nil && sc.syncOp.SyncStrategy.Apply != nil
}

func (sc *syncContext) isSelectiveSyncResourceOrAll(resourceState managedResource) bool {
	return sc.syncResources == nil ||
		(resourceState.Live != nil && argo.ContainsSyncResource(resourceState.Live.GetName(), resourceState.Live.GroupVersionKind(), sc.syncResources)) ||
		(resourceState.Target != nil && argo.ContainsSyncResource(resourceState.Target.GetName(), resourceState.Target.GroupVersionKind(), sc.syncResources))
}

// generateSyncTasks() generates the list of sync tasks we will be performing during this sync.
func (sc *syncContext) getSyncTasks() (tasks syncTasks, successful bool) {
	successful = true
	for _, resourceState := range sc.compareResult.managedResources {
		if sc.isSelectiveSyncResourceOrAll(resourceState) {
			obj := resourceState.Target
			if obj == nil {
				obj = resourceState.Live
			}

			// typically we'll have a single phase, but for some hooks, we may have more than one
			for _, syncPhase := range getSyncPhases(obj) {

				var targetObj *unstructured.Unstructured
				if resourceState.Target != nil {

					targetObj = resourceState.Target.DeepCopy()

					if targetObj.GetNamespace() == "" {
						// If target object's namespace is empty, we set namespace in the object. We do
						// this even though it might be a cluster-scoped resource. This prevents any
						// possibility of the resource from unintentionally becoming created in the
						// namespace during the `kubectl apply`
						targetObj.SetNamespace(sc.namespace)
					}

					// Hook resources names are deterministic, whether they are defined by the user (metadata.name),
					// or formulated at the time of the operation (metadata.generateName). If user specifies
					// metadata.generateName, then we will generate a formulated metadata.name before submission.
					if hook.IsHook(targetObj) && targetObj.GetName() == "" {
						postfix := strings.ToLower(fmt.Sprintf("%s-%s-%d", sc.syncRes.Revision[0:7], syncPhase, sc.opState.StartedAt.UTC().Unix()))
						generateName := targetObj.GetGenerateName()
						targetObj.SetName(fmt.Sprintf("%s%s", generateName, postfix))
					}
				}

				gvk := obj.GroupVersionKind()
				_, res := sc.syncRes.Resources.Find(gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName(), syncPhase)

				r := result{}

				if res != nil {
					r.sync = res.Status
					r.operation = res.OperationPhase
					r.message = res.Message
				}

				task := newSyncTask(syncPhase, resourceState.Live, targetObj, false, r)

				// this essentially enforces the old "apply" behaviour
				if task.isHook() && sc.skipHooks() {
					sc.log.WithFields(log.Fields{"task": task.String()}).Info("skipping hook")
					continue
				}

				// skip in-sync tasks
				if !task.isHook() && !resourceState.Diff.Modified {
					sc.log.WithFields(log.Fields{"task": task.String()}).Info("skipping in-sync resource")
					continue
				}

				serverRes, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)

				if err != nil {
					// Special case for custom resources: if CRD is not yet known by the K8s API server,
					// skip verification during `kubectl apply --dry-run` since we expect the CRD
					// to be created during app synchronization.
					if apierr.IsNotFound(err) && hasCRDOfGroupKind(sc.compareResult.managedResources, gvk.Group, gvk.Kind) {
						sc.log.WithFields(log.Fields{"task": task.String()}).Info("skip drip run for CRD")
						task.skipDryRun = true
					} else {
						sc.setResourceResult(&task, result{sync: ResultCodeSyncFailed, message: err.Error()})
						successful = false
					}
				} else {
					if !sc.proj.IsResourcePermitted(metav1.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, serverRes.Namespaced) {
						sc.setResourceResult(&task, result{sync: ResultCodeSyncFailed, message: fmt.Sprintf("Resource %s:%s is not permitted in project %s.", gvk.Group, gvk.Kind, sc.proj.Name)})
						successful = false
					}

					if serverRes.Namespaced && !sc.proj.IsDestinationPermitted(ApplicationDestination{Namespace: obj.GetNamespace(), Server: sc.server}) {
						sc.setResourceResult(&task, result{sync: ResultCodeSyncFailed, message: fmt.Sprintf("namespace %v is not permitted in project '%s'", targetObj.GetNamespace(), sc.proj.Name)})
						successful = false
					}
				}

				tasks = append(tasks, task)
			}
		}
	}

	sort.Sort(tasks)

	return tasks, successful
}

// applyObject performs a `kubectl apply` of a single resource
func (sc *syncContext) applyObject(targetObj *unstructured.Unstructured, dryRun bool, force bool) (resultCode ResultCode, message string) {
	message, err := sc.kubectl.ApplyResource(sc.config, targetObj, targetObj.GetNamespace(), dryRun, force)
	if err != nil {
		return ResultCodeSyncFailed, err.Error()
	}
	return ResultCodeSynced, message
}

// pruneObject deletes the object if both prune is true and dryRun is false. Otherwise appropriate message
func (sc *syncContext) pruneObject(liveObj *unstructured.Unstructured, prune, dryRun bool) (resultCode ResultCode, message string) {
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

// terminate looks for any running jobs/workflow hooks and deletes the resource
func (sc *syncContext) terminate() {
	terminateSuccessful := true
	sc.log.Info("terminating")
	tasks, _ := sc.getSyncTasks()
	for _, task := range tasks {
		if !task.isHook() || !task.result.operation.Completed() {
			continue
		}
		if isRunnable(task.groupVersionKind()) {
			sc.log.WithFields(log.Fields{"task": task.String()}).Info("deleting task")
			err := sc.deleteHook(task.getName(), task.getNamespace(), task.groupVersionKind())
			if err != nil {
				sc.setResourceResult(&task, result{operation: OperationFailed, message: fmt.Sprintf("Failed to delete: %v", err)})
				terminateSuccessful = false
			} else {
				sc.setResourceResult(&task, result{operation: OperationSucceeded, message: fmt.Sprintf("Deleted")})
			}
		}
	}
	if terminateSuccessful {
		sc.setOperationPhase(OperationFailed, "Operation terminated")
	} else {
		sc.setOperationPhase(OperationError, "Operation termination had errors")
	}
}

func (sc *syncContext) deleteHook(name, namespace string, gvk schema.GroupVersionKind) error {
	apiResource, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)
	if err != nil {
		return err
	}
	resource := kube.ToGroupVersionResource(gvk.GroupVersion().String(), apiResource)
	resIf := kube.ToResourceInterface(sc.dynamicIf, apiResource, resource, namespace)
	propagationPolicy := metav1.DeletePropagationForeground
	return resIf.Delete(name, &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}

func (sc *syncContext) runTasks(tasks syncTasks, dryRun bool) (successful bool) {

	dryRun = dryRun || sc.syncOp.DryRun

	sc.log.WithFields(log.Fields{"numTasks": len(tasks), "dryRun": dryRun}).Info("running tasks")

	successful = true
	var skipTasks syncTasks
	var createTasks syncTasks
	var pruneTasks syncTasks

	for _, task := range tasks {
		if hook.HasHook(task.getObj(), HookTypeSkip) {
			skipTasks = append(skipTasks, task)
		} else if task.isPrune() {
			pruneTasks = append(pruneTasks, task)
		} else {
			createTasks = append(createTasks, task)
		}
	}

	for _, task := range skipTasks {
		sc.log.WithFields(log.Fields{"task": task.String()}).Info("skipping")
		if !dryRun {
			sc.setResourceResult(&task, result{sync: ResultCodeSynced, message: "Skipped"})
		}
	}

	var wg sync.WaitGroup
	for _, task := range pruneTasks {
		wg.Add(1)
		go func(t syncTask) {
			defer wg.Done()
			sc.log.WithFields(log.Fields{"task": t.String()}).Info("pruning")
			status, message := sc.pruneObject(t.liveObj, sc.syncOp.Prune, dryRun)
			if status == ResultCodeSyncFailed {
				successful = false
			}
			if !dryRun || status == ResultCodeSyncFailed {
				sc.setResourceResult(&t, result{sync: status, message: message})
			}
		}(task)
	}
	wg.Wait()

	processCreateTasks := func(tasks []syncTask) {
		var createWg sync.WaitGroup
		for _, task := range tasks {
			if dryRun && task.skipDryRun {
				continue
			}
			createWg.Add(1)
			go func(t syncTask) {
				defer createWg.Done()
				sc.log.WithFields(log.Fields{"task": t.String()}).Info("applying")
				status, message := sc.applyObject(t.targetObj, dryRun, sc.syncOp.SyncStrategy.Force())
				if status == ResultCodeSyncFailed {
					successful = false
				}
				if !dryRun || status == ResultCodeSyncFailed {
					sc.setResourceResult(&t, result{sync: status, message: message})
				}
			}(task)
		}
		createWg.Wait()
	}

	var tasksGroup []syncTask
	for _, task := range createTasks {
		//Only wait if the type of the next task is different than the previous type
		if len(tasksGroup) > 0 && tasksGroup[0].targetObj.GetKind() != task.targetObj.GetKind() {
			processCreateTasks(tasksGroup)
			tasksGroup = []syncTask{task}
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
func (sc *syncContext) setResourceResult(task *syncTask, result result) {

	task.result = result

	sc.lock.Lock()
	defer sc.lock.Unlock()
	i, existing := sc.syncRes.Resources.Find(task.getGroup(), task.getKind(), task.getNamespace(), task.getName(), task.syncPhase)

	res := ResourceResult{
		Group:          task.getGroup(),
		Version:        task.getVersion(),
		Kind:           task.getKind(),
		Namespace:      task.getNamespace(),
		Name:           task.getName(),
		Status:         task.result.sync,
		Message:        task.result.message,
		OperationPhase: task.result.operation,
		SyncPhase:      task.syncPhase,
	}

	logCtx := sc.log.WithFields(log.Fields{"namespace": task.getNamespace(), "kind": task.getKind(), "name": task.getName()})

	if existing != nil {
		// update existing value
		if res.Status != existing.Status || res.OperationPhase != existing.OperationPhase {
			logCtx.Infof("updated resource operationPhase: %s -> %s", existing.OperationPhase, res.OperationPhase)
		}
		if res.Message != existing.Message {
			logCtx.Infof("updated resource message: %s -> %s", existing.Message, res.Message)
		}
		sc.syncRes.Resources[i] = res
	} else {
		logCtx.Infof("added resource resultCode: %s, operationPhase: %s, message: %s", res.Status, res.OperationPhase, res.Message)
		sc.syncRes.Resources = append(sc.syncRes.Resources, res)
	}
}

func (sc *syncContext) setOperationPhase(phase OperationPhase, message string) {
	if sc.opState.Phase != phase || sc.opState.Message != message {
		sc.log.Infof("Updating operation state. phase: %s -> %s, message: '%s' -> '%s'", sc.opState.Phase, phase, sc.opState.Message, message)
	}
	sc.opState.Phase = phase
	sc.opState.Message = message
}
