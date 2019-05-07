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
	appName       string
	proj          *AppProject
	compareResult *comparisonResult
	config        *rest.Config
	dynamicIf     dynamic.Interface
	disco         discovery.DiscoveryInterface
	kubectl       kube.Kubectl
	namespace     string
	server        string
	syncOp        *SyncOperation
	syncRes       *SyncOperationResult
	syncResources []SyncOperationResource
	opState       *OperationState
	log           *log.Entry
	isHealthy     func(obj unstructured.Unstructured) bool
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
		appName:       app.Name,
		proj:          proj,
		compareResult: compareResult,
		config:        restConfig,
		dynamicIf:     dynamicIf,
		disco:         disco,
		kubectl:       m.kubectl,
		namespace:     app.Spec.Destination.Namespace,
		server:        app.Spec.Destination.Server,
		syncOp:        &syncOp,
		syncRes:       syncRes,
		syncResources: syncResources,
		opState:       state,
		log:           log.WithFields(log.Fields{"application": app.Name}),
		isHealthy: func(obj unstructured.Unstructured) bool {
			resourceHealth, err := health.GetResourceHealth(&obj, m.settings.ResourceOverrides)
			if err != nil {
				log.WithFields(log.Fields{"err": err}).Warn("error determining health, assuming un-healthy")
				return false
			}
			return resourceHealth != nil && resourceHealth.Status == HealthStatusHealthy
		},
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

// sync has performs the actual apply or hook based sync
func (sc *syncContext) sync() {
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
		// Optimization: we only wish to do this once per operation, performing additional dry-runs
		// is harmless, but redundant. The indicator we use to detect if we have already performed
		// the dry-run for this operation, is if the resource or hook list is empty.
		if !sc.executeTasks(tasks, true) {
			sc.setOperationPhase(OperationFailed, "one or more objects failed to apply (dry run)")
			return
		}
	}

	// remove started tasks
	tasks = tasks.Filter(func(task syncTask) bool {
		obj := task.getObj()
		gvk := obj.GroupVersionKind()
		_, result := sc.syncRes.Resources.Find(gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName(), task.syncPhase)
		return result == nil
	})

	// remove any tasks not in this wave
	if len(tasks) > 0 {
		syncPhase := tasks[0].syncPhase
		wave := tasks[0].getWave()
		sc.log.WithFields(log.Fields{"syncPhase": syncPhase, "wave": wave}).Info("syncing phase/wave")
		tasks = tasks.Filter(func(task syncTask) bool {
			return task.syncPhase == syncPhase && task.getWave() == wave
		})
		if len(tasks) == 0 {
			panic("this can never happen")
		}
	}

	// If no sync tasks were generated (e.g., in case all application manifests have been removed),
	// set the sync operation as successful.
	if len(tasks) == 0 {
		sc.setOperationPhase(OperationSucceeded, "successfully synced")
		return
	}

	if !sc.executeTasks(tasks, false) {
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
func (sc *syncContext) getSyncTasks() (syncTasks syncTasks, successful bool) {
	successful = true
	for _, resourceState := range sc.compareResult.managedResources {
		if sc.isSelectiveSyncResourceOrAll(resourceState) {
			obj := resourceState.Target
			if obj == nil {
				obj = resourceState.Live
			}

			// this essentially enforces the old "apply" behaviour
			if hook.IsArgoHook(obj) && sc.skipHooks() {
				continue
			}

			// typically we'll have a single phase, but for some hooks, we may have more than one
			for _, syncPhase := range getSyncPhases(obj) {

				var targetObj *unstructured.Unstructured
				skipDryRun := false
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

					gvk := targetObj.GroupVersionKind()
					serverRes, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)

					if err != nil {
						// Special case for custom resources: if CRD is not yet known by the K8s API server,
						// skip verification during `kubectl apply --dry-run` since we expect the CRD
						// to be created during app synchronization.
						if apierr.IsNotFound(err) && hasCRDOfGroupKind(sc.compareResult.managedResources, gvk.Group, gvk.Kind) {
							skipDryRun = true
						} else {
							sc.setResourceResultByObj(targetObj, syncPhase, ResultCodeSyncFailed, err.Error())
							successful = false
						}
					} else {
						if !sc.proj.IsResourcePermitted(metav1.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, serverRes.Namespaced) {
							sc.setResourceResultByObj(targetObj, syncPhase, ResultCodeSyncFailed, fmt.Sprintf("Resource %s:%s is not permitted in project %s.", gvk.Group, gvk.Kind, sc.proj.Name))
							successful = false
						}

						if serverRes.Namespaced && !sc.proj.IsDestinationPermitted(ApplicationDestination{Namespace: targetObj.GetNamespace(), Server: sc.server}) {
							sc.setResourceResultByObj(targetObj, syncPhase, ResultCodeSyncFailed, fmt.Sprintf("namespace %v is not permitted in project '%s'", targetObj.GetNamespace(), sc.proj.Name))
							successful = false
						}
					}
				}

				syncTasks = append(syncTasks, newSyncTask(syncPhase, resourceState.Live, targetObj, skipDryRun))
			}
		}
	}

	sort.Sort(syncTasks)

	return syncTasks, successful
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

	for _, res := range sc.syncRes.Resources {
		if !res.IsHook || res.Completed() {
			continue
		}
		if isRunnable(&res) {
			err := sc.deleteHook(res.Name, res.Namespace, res.GroupVersionKind())
			if err != nil {
				sc.setResourceResult(res.GroupVersionKind(), res.Namespace, res.Name, res.SyncPhase, ResultCodeSyncFailed, fmt.Sprintf("Failed to delete %s hook %s/%s: %v", res.SyncPhase, res.Kind, res.Name, err))
				terminateSuccessful = false
			} else {
				sc.setResourceResult(res.GroupVersionKind(), res.Namespace, res.Name, res.SyncPhase, ResultCodeSyncFailed, fmt.Sprintf("Deleted %s hook %s/%s", res.SyncPhase, res.Kind, res.Name))
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

func (sc *syncContext) executeTasks(tasks syncTasks, dryRun bool) (successful bool) {

	successful = true
	dryRun = dryRun || sc.syncOp.DryRun

	var createTasks syncTasks
	var pruneTasks syncTasks

	for _, task := range tasks {

		sc.log.WithFields(log.Fields{"resource": task.getObj().GetName(), "syncPhase": task.syncPhase}).Info("task will be run shortly")

		if hook.HasHook(task.getObj(), HookTypeSkip) {
			if !dryRun {
				sc.setResourceResultByObj(task.liveObj, task.syncPhase, ResultCodeSynced, "Skipped")
			}
			continue
		}

		if task.isPrune() {
			pruneTasks = append(pruneTasks, task)
		} else {
			createTasks = append(createTasks, task)
		}
	}

	var wg sync.WaitGroup
	for _, task := range pruneTasks {
		wg.Add(1)
		go func(t syncTask) {
			defer wg.Done()
			resultCode, message := sc.pruneObject(t.liveObj, sc.syncOp.Prune, dryRun)
			if resultCode == ResultCodeSyncFailed {
				successful = false
			}
			if !dryRun || resultCode == ResultCodeSyncFailed {
				sc.setResourceResultByObj(t.liveObj, t.syncPhase, resultCode, message)
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
				resultCode, message := sc.applyObject(t.targetObj, dryRun, sc.syncOp.SyncStrategy.Force())
				if resultCode == ResultCodeSyncFailed {
					successful = false
				}
				if !dryRun || resultCode == ResultCodeSyncFailed {
					sc.setResourceResultByObj(t.targetObj, t.syncPhase, resultCode, message)
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
func (sc *syncContext) setResourceResult(gvk schema.GroupVersionKind, namespace, name string, syncPhase SyncPhase, result ResultCode, message string) {

	res := ResourceResult{
		Group:     gvk.Group,
		Version:   gvk.Version,
		Kind:      gvk.Kind,
		Namespace: namespace,
		Name:      name,
		Status:    result,
		Message:   message,
		SyncPhase: syncPhase,
	}

	sc.lock.Lock()
	defer sc.lock.Unlock()
	i, existing := sc.syncRes.Resources.Find(gvk.Group, gvk.Kind, namespace, name, syncPhase)

	if existing != nil {
		// update existing value
		if res.Status != result {
			sc.log.Infof("updated resource %s/%s/%s result: %s -> %s", gvk.Kind, namespace, name, res.Status, result)
		}
		if res.Message != message {
			sc.log.Infof("updated resource %s/%s/%s message: %s -> %s", gvk.Kind, namespace, name, res.Message, message)
		}
		sc.syncRes.Resources[i] = res
	} else {
		sc.log.Infof("added resource %s/%s result: %s, message: %s", gvk.Kind, name, res.Status, res.Message)
		sc.syncRes.Resources = append(sc.syncRes.Resources, res)
	}
}

func (sc *syncContext) setResourceResultByObj(obj *unstructured.Unstructured, syncPhase SyncPhase, result ResultCode, message string) {
	sc.setResourceResult(obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName(), syncPhase, result, message)
}

func (sc *syncContext) setOperationPhase(phase OperationPhase, message string) {
	if sc.opState.Phase != phase || sc.opState.Message != message {
		sc.log.Infof("Updating operation state. phase: %s -> %s, message: '%s' -> '%s'", sc.opState.Phase, phase, sc.opState.Message, message)
	}
	sc.opState.Phase = phase
	sc.opState.Message = message
}
