package controller

import (
	"context"
	"fmt"
	"sort"
	"sync"

	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/controller/metrics"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo"
	hookutil "github.com/argoproj/argo-cd/util/hook"
	"github.com/argoproj/argo-cd/util/kube"
)

type syncContext struct {
	appName       string
	proj          *appv1.AppProject
	compareResult *comparisonResult
	config        *rest.Config
	dynamicIf     dynamic.Interface
	disco         discovery.DiscoveryInterface
	kubectl       kube.Kubectl
	namespace     string
	server        string
	syncOp        *appv1.SyncOperation
	syncRes       *appv1.SyncOperationResult
	syncResources []appv1.SyncOperationResource
	opState       *appv1.OperationState
	log           *log.Entry
	// lock to protect concurrent updates of the result list
	lock sync.Mutex
}

func (m *appStateManager) SyncAppState(app *appv1.Application, state *appv1.OperationState) {
	// Sync requests might be requested with ambiguous revisions (e.g. master, HEAD, v1.2.3).
	// This can change meaning when resuming operations (e.g a hook sync). After calculating a
	// concrete git commit SHA, the SHA is remembered in the status.operationState.syncResult field.
	// This ensures that when resuming an operation, we sync to the same revision that we initially
	// started with.
	var revision string
	var syncOp appv1.SyncOperation
	var syncRes *appv1.SyncOperationResult
	var syncResources []appv1.SyncOperationResource
	var source appv1.ApplicationSource

	if state.Operation.Sync == nil {
		state.Phase = appv1.OperationFailed
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
		syncRes = &appv1.SyncOperationResult{}
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
		state.Phase = appv1.OperationError
		state.Message = err.Error()
		return
	}

	// If there are any error conditions, do not perform the operation
	errConditions := make([]appv1.ApplicationCondition, 0)
	for i := range compareResult.conditions {
		if compareResult.conditions[i].IsError() {
			errConditions = append(errConditions, compareResult.conditions[i])
		}
	}
	if len(errConditions) > 0 {
		state.Phase = appv1.OperationError
		state.Message = argo.FormatAppConditions(errConditions)
		return
	}

	// We now have a concrete commit SHA. Save this in the sync result revision so that we remember
	// what we should be syncing to when resuming operations.
	syncRes.Revision = compareResult.syncStatus.Revision

	clst, err := m.db.GetCluster(context.Background(), app.Spec.Destination.Server)
	if err != nil {
		state.Phase = appv1.OperationError
		state.Message = err.Error()
		return
	}

	restConfig := metrics.AddMetricsTransportWrapper(m.metricsServer, app, clst.RESTConfig())
	dynamicIf, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		state.Phase = appv1.OperationError
		state.Message = fmt.Sprintf("Failed to initialize dynamic client: %v", err)
		return
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		state.Phase = appv1.OperationError
		state.Message = fmt.Sprintf("Failed to initialize discovery client: %v", err)
		return
	}

	proj, err := argo.GetAppProject(&app.Spec, v1alpha1.NewAppProjectLister(m.projInformer.GetIndexer()), m.namespace)
	if err != nil {
		state.Phase = appv1.OperationError
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
	}

	if state.Phase == appv1.OperationTerminating {
		syncCtx.terminate()
	} else {
		syncCtx.sync()
	}

	if !syncOp.DryRun && len(syncOp.Resources) == 0 && syncCtx.opState.Phase.Successful() {
		err := m.persistRevisionHistory(app, compareResult.syncStatus.Revision, source)
		if err != nil {
			state.Phase = appv1.OperationError
			state.Message = fmt.Sprintf("failed to record sync to history: %v", err)
		}
	}
}

// sync has performs the actual apply or hook based sync
func (sc *syncContext) sync() {
	syncTasks, successful := sc.generateSyncTasks()
	if !successful {
		sc.setOperationPhase(appv1.OperationFailed, "one or more synchronization tasks are not valid")
		return
	}

	// If no sync tasks were generated (e.g., in case all application manifests have been removed),
	// set the sync operation as successful.
	if len(syncTasks) == 0 {
		sc.setOperationPhase(appv1.OperationSucceeded, "successfully synced (no manifests)")
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

// generateSyncTasks() generates the list of sync tasks we will be performing during this sync.
func (sc *syncContext) generateSyncTasks() (syncTasks, bool) {
	var syncTasks syncTasks
	successful := true
	for _, resourceState := range sc.compareResult.managedResources {
		if resourceState.Hook {
			continue
		}
		if sc.syncResources == nil ||
			(resourceState.Live != nil && argo.ContainsSyncResource(resourceState.Live.GetName(), resourceState.Live.GroupVersionKind(), sc.syncResources)) ||
			(resourceState.Target != nil && argo.ContainsSyncResource(resourceState.Target.GetName(), resourceState.Target.GroupVersionKind(), sc.syncResources)) {

			skipDryRun := false
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
				gvk := targetObj.GroupVersionKind()
				serverRes, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)

				if err != nil {
					// Special case for custom resources: if CRD is not yet known by the K8s API server,
					// skip verification during `kubectl apply --dry-run` since we expect the CRD
					// to be created during app synchronization.
					if apierr.IsNotFound(err) && hasCRDOfGroupKind(sc.compareResult.managedResources, gvk.Group, gvk.Kind) {
						skipDryRun = true
					} else {
						sc.setResourceDetails(&appv1.ResourceResult{
							Name:      targetObj.GetName(),
							Group:     gvk.Group,
							Version:   gvk.Version,
							Kind:      targetObj.GetKind(),
							Namespace: targetObj.GetNamespace(),
							Message:   err.Error(),
							Status:    appv1.ResultCodeSyncFailed,
						})
						successful = false
					}
				} else {
					if !sc.proj.IsResourcePermitted(metav1.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, serverRes.Namespaced) {
						sc.setResourceDetails(&appv1.ResourceResult{
							Name:      targetObj.GetName(),
							Group:     gvk.Group,
							Version:   gvk.Version,
							Kind:      targetObj.GetKind(),
							Namespace: targetObj.GetNamespace(),
							Message:   fmt.Sprintf("Resource %s:%s is not permitted in project %s.", gvk.Group, gvk.Kind, sc.proj.Name),
							Status:    appv1.ResultCodeSyncFailed,
						})
						successful = false
					}

					if serverRes.Namespaced && !sc.proj.IsDestinationPermitted(appv1.ApplicationDestination{Namespace: targetObj.GetNamespace(), Server: sc.server}) {
						sc.setResourceDetails(&appv1.ResourceResult{
							Name:      targetObj.GetName(),
							Group:     gvk.Group,
							Version:   gvk.Version,
							Kind:      targetObj.GetKind(),
							Namespace: targetObj.GetNamespace(),
							Message:   fmt.Sprintf("namespace %v is not permitted in project '%s'", targetObj.GetNamespace(), sc.proj.Name),
							Status:    appv1.ResultCodeSyncFailed,
						})
						successful = false
					}
				}
			}
			syncTask := syncTask{
				liveObj:    resourceState.Live,
				targetObj:  targetObj,
				skipDryRun: skipDryRun,
				modified:   resourceState.Diff.Modified,
			}
			syncTasks = append(syncTasks, syncTask)
		}
	}

	sort.Sort(syncTasks)

	return syncTasks, successful
}

// startedPreSyncPhase detects if we already started the PreSync stage of a sync operation.
// This is equal to if we have anything in our resource or hook list
func (sc *syncContext) startedPreSyncPhase() bool {
	return len(sc.syncRes.Resources) > 0
}

// startedSyncPhase detects if we have already started the Sync stage of a sync operation.
// This is equal to if the resource list is non-empty, or we we see Sync/PostSync hooks
func (sc *syncContext) startedSyncPhase() bool {
	postponed := sc.postponed()
	for _, res := range sc.syncRes.Resources {
		if !res.IsHook() && !postponed {
			return true
		}
		if res.HookType == appv1.HookTypeSync || res.HookType == appv1.HookTypePostSync {
			return true
		}
	}
	return false
}

// startedPostSyncPhase detects if we have already started the PostSync stage. This is equal to if
// we see any PostSync hooks
func (sc *syncContext) startedPostSyncPhase() bool {
	for _, res := range sc.syncRes.Resources {
		if res.IsHook() && res.HookType == appv1.HookTypePostSync {
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
func (sc *syncContext) applyObject(targetObj *unstructured.Unstructured, dryRun bool, force bool) appv1.ResourceResult {
	gvk := targetObj.GroupVersionKind()
	resDetails := appv1.ResourceResult{
		Name:      targetObj.GetName(),
		Group:     gvk.Group,
		Version:   gvk.Version,
		Kind:      targetObj.GetKind(),
		Namespace: targetObj.GetNamespace(),
	}
	message, err := sc.kubectl.ApplyResource(sc.config, targetObj, targetObj.GetNamespace(), dryRun, force)
	if err != nil {
		resDetails.Message = err.Error()
		resDetails.Status = appv1.ResultCodeSyncFailed
		return resDetails
	}

	resDetails.Message = message
	resDetails.Status = appv1.ResultCodeSynced
	return resDetails
}

// pruneObject deletes the object if both prune is true and dryRun is false. Otherwise appropriate message
func (sc *syncContext) pruneObject(liveObj *unstructured.Unstructured, prune, dryRun bool) appv1.ResourceResult {
	gvk := liveObj.GroupVersionKind()
	resDetails := appv1.ResourceResult{
		Name:      liveObj.GetName(),
		Group:     gvk.Group,
		Version:   gvk.Version,
		Kind:      liveObj.GetKind(),
		Namespace: liveObj.GetNamespace(),
	}
	if prune {
		if dryRun {
			resDetails.Message = "pruned (dry run)"
			resDetails.Status = appv1.ResultCodePruned
		} else {
			resDetails.Message = "pruned"
			resDetails.Status = appv1.ResultCodePruned
			// Skip deletion if object is already marked for deletion, so we don't cause a resource update hotloop
			deletionTimestamp := liveObj.GetDeletionTimestamp()
			if deletionTimestamp == nil || deletionTimestamp.IsZero() {
				err := sc.kubectl.DeleteResource(sc.config, liveObj.GroupVersionKind(), liveObj.GetName(), liveObj.GetNamespace(), false)
				if err != nil {
					resDetails.Message = err.Error()
					resDetails.Status = appv1.ResultCodeSyncFailed
				}
			}
		}
	} else {
		resDetails.Message = "ignored (requires pruning)"
		resDetails.Status = appv1.ResultCodePruneSkipped
	}
	return resDetails
}

func hasCRDOfGroupKind(resources []managedResource, group string, kind string) bool {
	for _, res := range resources {
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

// performs a apply based sync of the given sync tasks (possibly pruning the objects).
// If update is true, will updates the resource details with the result.
// Or if the prune/apply failed, will also update the result.
func (sc *syncContext) doApplySync(tasks syncTasks, dryRun, force, update bool) bool {

	syncSuccessful := true

	var createTasks syncTasks
	var pruneTasks syncTasks
	sort.Sort(tasks)

	var nextWave = tasks.getNextWave()
	log.WithFields(log.Fields{"nextWave": nextWave, "dryRun": dryRun, "tasks": tasks}).Debug("apply sync")
	for _, task := range tasks {
		wave := task.getWave()
		if !dryRun && wave > nextWave {
			log.WithFields(log.Fields{"kind": task.targetObj.GetKind(), "name": task.targetObj.GetName(), "nextWave": nextWave, "wave": wave}).Debug("skipping, not in next wave")

			liveObj := task.liveObj
			gvk := task.liveObj.GroupVersionKind()
			sc.syncRes.Resources = append(sc.syncRes.Resources,
				&appv1.ResourceResult{
					Name:      liveObj.GetName(),
					Group:     gvk.Group,
					Version:   gvk.Version,
					Kind:      liveObj.GetKind(),
					Namespace: liveObj.GetNamespace(),
					Status:    appv1.ResultCodePostponed,
				})
			continue
		}
		if task.targetObj == nil {
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
			resDetails := sc.pruneObject(t.liveObj, sc.syncOp.Prune, dryRun)
			if !resDetails.Status.Successful() {
				syncSuccessful = false
			}
			if update || !resDetails.Status.Successful() {
				sc.setResourceDetails(&resDetails)
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
				if hookutil.IsHook(t.targetObj) {
					return
				}
				resDetails := sc.applyObject(t.targetObj, dryRun, force)
				if !resDetails.Status.Successful() {
					syncSuccessful = false
				}
				if update || !resDetails.Status.Successful() {
					sc.setResourceDetails(&resDetails)
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
	return syncSuccessful
}

// setResourceDetails sets a resource details in the SyncResult.Resources list
func (sc *syncContext) setResourceDetails(details *appv1.ResourceResult) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	for i, res := range sc.syncRes.Resources {
		if res.Group == details.Group && res.Kind == details.Kind && res.Namespace == details.Namespace && res.Name == details.Name {
			// update existing value
			if res.Status != details.Status {
				sc.log.Infof("updated resource %s/%s/%s status: %s -> %s", res.Kind, res.Namespace, res.Name, res.Status, details.Status)
			}
			if res.Message != details.Message {
				sc.log.Infof("updated resource %s/%s/%s message: %s -> %s", res.Kind, res.Namespace, res.Name, res.Message, details.Message)
			}
			sc.syncRes.Resources[i] = details
			return
		}
	}
	sc.log.Infof("added resource %s/%s status: %s, message: %s", details.Kind, details.Name, details.Status, details.Message)
	sc.syncRes.Resources = append(sc.syncRes.Resources, details)
}

func (sc *syncContext) postponed() bool {
	for _, resource := range sc.syncRes.Resources {
		if resource.Status == appv1.ResultCodePostponed {
			return true
		}
	}
	return false
}
