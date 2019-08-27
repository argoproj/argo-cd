package controller

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller/metrics"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	listersv1alpha1 "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/health"
	"github.com/argoproj/argo-cd/util/hook"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/resource"
)

const (
	crdReadinessTimeout = time.Duration(3) * time.Second
)

type syncContext struct {
	resourceOverrides   map[string]v1alpha1.ResourceOverride
	appName             string
	proj                *v1alpha1.AppProject
	compareResult       *comparisonResult
	config              *rest.Config
	dynamicIf           dynamic.Interface
	disco               discovery.DiscoveryInterface
	extensionsclientset *clientset.Clientset
	kubectl             kube.Kubectl
	namespace           string
	server              string
	syncOp              *v1alpha1.SyncOperation
	syncRes             *v1alpha1.SyncOperationResult
	syncResources       []v1alpha1.SyncOperationResource
	opState             *v1alpha1.OperationState
	log                 *log.Entry
	// lock to protect concurrent updates of the result list
	lock sync.Mutex
}

func (m *appStateManager) SyncAppState(app *v1alpha1.Application, state *v1alpha1.OperationState) {
	// Sync requests might be requested with ambiguous revisions (e.g. master, HEAD, v1.2.3).
	// This can change meaning when resuming operations (e.g a hook sync). After calculating a
	// concrete git commit SHA, the SHA is remembered in the status.operationState.syncResult field.
	// This ensures that when resuming an operation, we sync to the same revision that we initially
	// started with.
	var revision string
	var syncOp v1alpha1.SyncOperation
	var syncRes *v1alpha1.SyncOperationResult
	var syncResources []v1alpha1.SyncOperationResource
	var source v1alpha1.ApplicationSource

	if state.Operation.Sync == nil {
		state.Phase = v1alpha1.OperationFailed
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
		syncRes = &v1alpha1.SyncOperationResult{}
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

	compareResult := m.CompareAppState(app, revision, source, false, syncOp.Manifests)

	// If there are any error conditions, do not perform the operation
	errConditions := make([]v1alpha1.ApplicationCondition, 0)
	for i := range compareResult.conditions {
		if compareResult.conditions[i].IsError() {
			errConditions = append(errConditions, compareResult.conditions[i])
		}
	}
	if len(errConditions) > 0 {
		state.Phase = v1alpha1.OperationError
		state.Message = argo.FormatAppConditions(errConditions)
		return
	}

	// We now have a concrete commit SHA. Save this in the sync result revision so that we remember
	// what we should be syncing to when resuming operations.
	syncRes.Revision = compareResult.syncStatus.Revision

	clst, err := m.db.GetCluster(context.Background(), app.Spec.Destination.Server)
	if err != nil {
		state.Phase = v1alpha1.OperationError
		state.Message = err.Error()
		return
	}

	restConfig := metrics.AddMetricsTransportWrapper(m.metricsServer, app, clst.RESTConfig())
	dynamicIf, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		state.Phase = v1alpha1.OperationError
		state.Message = fmt.Sprintf("Failed to initialize dynamic client: %v", err)
		return
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		state.Phase = v1alpha1.OperationError
		state.Message = fmt.Sprintf("Failed to initialize discovery client: %v", err)
		return
	}

	extensionsclientset, err := clientset.NewForConfig(restConfig)
	if err != nil {
		state.Phase = v1alpha1.OperationError
		state.Message = fmt.Sprintf("Failed to initialize extensions client: %v", err)
		return
	}

	proj, err := argo.GetAppProject(&app.Spec, listersv1alpha1.NewAppProjectLister(m.projInformer.GetIndexer()), m.namespace)
	if err != nil {
		state.Phase = v1alpha1.OperationError
		state.Message = fmt.Sprintf("Failed to load application project: %v", err)
		return
	}

	resourceOverrides, err := m.settingsMgr.GetResourceOverrides()
	if err != nil {
		state.Phase = v1alpha1.OperationError
		state.Message = fmt.Sprintf("Failed to load resource overrides: %v", err)
		return
	}

	syncCtx := syncContext{
		resourceOverrides:   resourceOverrides,
		appName:             app.Name,
		proj:                proj,
		compareResult:       compareResult,
		config:              restConfig,
		dynamicIf:           dynamicIf,
		disco:               disco,
		extensionsclientset: extensionsclientset,
		kubectl:             m.kubectl,
		namespace:           app.Spec.Destination.Namespace,
		server:              app.Spec.Destination.Server,
		syncOp:              &syncOp,
		syncRes:             syncRes,
		syncResources:       syncResources,
		opState:             state,
		log:                 log.WithFields(log.Fields{"application": app.Name}),
	}

	if state.Phase == v1alpha1.OperationTerminating {
		syncCtx.terminate()
	} else {
		syncCtx.sync()
	}

	syncCtx.log.Info("sync/terminate complete")

	if !syncOp.DryRun && !syncCtx.isSelectiveSync() && syncCtx.opState.Phase.Successful() {
		err := m.persistRevisionHistory(app, compareResult.syncStatus.Revision, source)
		if err != nil {
			syncCtx.setOperationPhase(v1alpha1.OperationError, fmt.Sprintf("failed to record sync to history: %v", err))
		}
	}
}

// sync has performs the actual apply or hook based sync
func (sc *syncContext) sync() {
	sc.log.WithFields(log.Fields{"isSelectiveSync": sc.isSelectiveSync(), "skipHooks": sc.skipHooks(), "started": sc.started()}).Info("syncing")
	tasks, successful := sc.getSyncTasks()
	if !successful {
		sc.setOperationPhase(v1alpha1.OperationFailed, "one or more synchronization tasks are not valid")
		return
	}

	sc.log.WithFields(log.Fields{"tasks": tasks, "isSelectiveSync": sc.isSelectiveSync()}).Info("tasks")

	// Perform a `kubectl apply --dry-run` against all the manifests. This will detect most (but
	// not all) validation issues with the user's manifests (e.g. will detect syntax issues, but
	// will not not detect if they are mutating immutable fields). If anything fails, we will refuse
	// to perform the sync. we only wish to do this once per operation, performing additional dry-runs
	// is harmless, but redundant. The indicator we use to detect if we have already performed
	// the dry-run for this operation, is if the resource or hook list is empty.
	if !sc.started() {
		sc.log.Debug("dry-run")
		if !sc.runTasks(tasks, true) {
			sc.setOperationPhase(v1alpha1.OperationFailed, "one or more objects failed to apply (dry run)")
			return
		}
	}

	// update status of any tasks that are running, note that this must exclude pruning tasks
	for _, task := range tasks.Filter(func(t *syncTask) bool {
		// just occasionally, you can be running yet not have a live resource
		return t.running() && t.liveObj != nil
	}) {
		if task.isHook() {
			// update the hook's result
			operationState, message := getOperationPhase(task.liveObj)
			sc.setResourceResult(task, "", operationState, message)

			// maybe delete the hook
			if enforceHookDeletePolicy(task.liveObj, task.operationState) {
				err := sc.deleteResource(task)
				if err != nil {
					sc.setResourceResult(task, "", v1alpha1.OperationError, fmt.Sprintf("failed to delete resource: %v", err))
				}
			}
		} else {
			// this must be calculated on the live object
			healthStatus, err := health.GetResourceHealth(task.liveObj, sc.resourceOverrides)
			if err == nil {
				log.WithFields(log.Fields{"task": task, "healthStatus": healthStatus}).Debug("attempting to update health of running task")
				if healthStatus == nil {
					// some objects (e.g. secret) do not have health, and they automatically success
					sc.setResourceResult(task, task.syncStatus, v1alpha1.OperationSucceeded, task.message)
				} else {
					switch healthStatus.Status {
					case v1alpha1.HealthStatusHealthy:
						sc.setResourceResult(task, task.syncStatus, v1alpha1.OperationSucceeded, healthStatus.Message)
					case v1alpha1.HealthStatusDegraded:
						sc.setResourceResult(task, task.syncStatus, v1alpha1.OperationFailed, healthStatus.Message)
					}
				}
			}
		}
	}

	// if (a) we are multi-step and we have any running tasks,
	// or (b) there are any running hooks,
	// then wait...
	multiStep := tasks.multiStep()
	if tasks.Any(func(t *syncTask) bool { return (multiStep || t.isHook()) && t.running() }) {
		sc.setOperationPhase(v1alpha1.OperationRunning, "one or more tasks are running")
		return
	}

	// syncFailTasks only run during failure, so separate them from regular tasks
	syncFailTasks, tasks := tasks.Split(func(t *syncTask) bool { return t.phase == v1alpha1.SyncPhaseSyncFail })

	// if there are any completed but unsuccessful tasks, sync is a failure.
	if tasks.Any(func(t *syncTask) bool { return t.completed() && !t.successful() }) {
		sc.setOperationFailed(syncFailTasks, "one or more synchronization tasks completed unsuccessfully")
		return
	}

	sc.log.WithFields(log.Fields{"tasks": tasks}).Debug("filtering out non-pending tasks")
	// remove tasks that are completed, we can assume that there are no running tasks
	tasks = tasks.Filter(func(t *syncTask) bool { return t.pending() })

	// If no sync tasks were generated (e.g., in case all application manifests have been removed),
	// the sync operation is successful.
	if len(tasks) == 0 {
		sc.setOperationPhase(v1alpha1.OperationSucceeded, "successfully synced (no more tasks)")
		return
	}

	// remove any tasks not in this wave
	phase := tasks.phase()
	wave := tasks.wave()

	// if it is the last phase/wave and the only remaining tasks are non-hooks, the we are successful
	// EVEN if those objects subsequently degraded
	// This handles the common case where neither hooks or waves are used and a sync equates to simply an (asynchronous) kubectl apply of manifests, which succeeds immediately.
	complete := !tasks.Any(func(t *syncTask) bool { return t.phase != phase || wave != t.wave() || t.isHook() })

	sc.log.WithFields(log.Fields{"phase": phase, "wave": wave, "tasks": tasks, "syncFailTasks": syncFailTasks}).Debug("filtering tasks in correct phase and wave")
	tasks = tasks.Filter(func(t *syncTask) bool { return t.phase == phase && t.wave() == wave })

	sc.setOperationPhase(v1alpha1.OperationRunning, "one or more tasks are running")

	sc.log.WithFields(log.Fields{"tasks": tasks}).Debug("wet-run")
	if !sc.runTasks(tasks, false) {
		sc.setOperationFailed(syncFailTasks, "one or more objects failed to apply")
	} else {
		if complete {
			sc.setOperationPhase(v1alpha1.OperationSucceeded, "successfully synced (all tasks run)")
		}
	}
}

func (sc *syncContext) setOperationFailed(syncFailTasks syncTasks, message string) {
	if len(syncFailTasks) > 0 {
		// if all the failure hooks are completed, don't run them again, and mark the sync as failed
		if syncFailTasks.All(func(task *syncTask) bool { return task.completed() }) {
			sc.setOperationPhase(v1alpha1.OperationFailed, message)
			return
		}
		// otherwise, we need to start the failure hooks, and then return without setting
		// the phase, so we make sure we have at least one more sync
		sc.log.WithFields(log.Fields{"syncFailTasks": syncFailTasks}).Debug("running sync fail tasks")
		if !sc.runTasks(syncFailTasks, false) {
			sc.setOperationPhase(v1alpha1.OperationFailed, message)
		}
	} else {
		sc.setOperationPhase(v1alpha1.OperationFailed, message)
	}
}

func (sc *syncContext) started() bool {
	return len(sc.syncRes.Resources) > 0
}

func (sc *syncContext) isSelectiveSync() bool {
	// we've selected no resources
	if sc.syncResources == nil {
		return false
	}

	// map both lists into string
	var a []string
	for _, r := range sc.compareResult.resources {
		if !r.Hook {
			a = append(a, fmt.Sprintf("%s:%s:%s", r.Group, r.Kind, r.Name))
		}
	}
	sort.Strings(a)

	var b []string
	for _, r := range sc.syncResources {
		b = append(b, fmt.Sprintf("%s:%s:%s", r.Group, r.Kind, r.Name))
	}
	sort.Strings(b)

	return !reflect.DeepEqual(a, b)
}

// this essentially enforces the old "apply" behaviour
func (sc *syncContext) skipHooks() bool {
	// All objects passed a `kubectl apply --dry-run`, so we are now ready to actually perform the sync.
	// default sync strategy to hook if no strategy
	return sc.syncOp.IsApplyStrategy() || sc.isSelectiveSync()
}

func (sc *syncContext) containsResource(resourceState managedResource) bool {
	return !sc.isSelectiveSync() ||
		(resourceState.Live != nil && argo.ContainsSyncResource(resourceState.Live.GetName(), resourceState.Live.GroupVersionKind(), sc.syncResources)) ||
		(resourceState.Target != nil && argo.ContainsSyncResource(resourceState.Target.GetName(), resourceState.Target.GroupVersionKind(), sc.syncResources))
}

// generates the list of sync tasks we will be performing during this sync.
func (sc *syncContext) getSyncTasks() (_ syncTasks, successful bool) {
	resourceTasks := syncTasks{}
	successful = true

	for _, resource := range sc.compareResult.managedResources {
		if !sc.containsResource(resource) {
			log.WithFields(log.Fields{"group": resource.Group, "kind": resource.Kind, "name": resource.Name}).
				Debug("skipping")
			continue
		}

		obj := obj(resource.Target, resource.Live)

		// this creates garbage tasks
		if hook.IsHook(obj) {
			log.WithFields(log.Fields{"group": obj.GroupVersionKind().Group, "kind": obj.GetKind(), "namespace": obj.GetNamespace(), "name": obj.GetName()}).
				Debug("skipping hook")
			continue
		}

		for _, phase := range syncPhases(obj) {
			resourceTasks = append(resourceTasks, &syncTask{phase: phase, targetObj: resource.Target, liveObj: resource.Live})
		}
	}

	sc.log.WithFields(log.Fields{"resourceTasks": resourceTasks}).Debug("tasks from managed resources")

	hookTasks := syncTasks{}
	if !sc.skipHooks() {
		for _, obj := range sc.compareResult.hooks {
			for _, phase := range syncPhases(obj) {
				// Hook resources names are deterministic, whether they are defined by the user (metadata.name),
				// or formulated at the time of the operation (metadata.generateName). If user specifies
				// metadata.generateName, then we will generate a formulated metadata.name before submission.
				targetObj := obj.DeepCopy()
				if targetObj.GetName() == "" {
					postfix := strings.ToLower(fmt.Sprintf("%s-%s-%d", sc.syncRes.Revision[0:7], phase, sc.opState.StartedAt.UTC().Unix()))
					generateName := obj.GetGenerateName()
					targetObj.SetName(fmt.Sprintf("%s%s", generateName, postfix))
				}

				hookTasks = append(hookTasks, &syncTask{phase: phase, targetObj: targetObj})
			}
		}
	}

	sc.log.WithFields(log.Fields{"hookTasks": hookTasks}).Debug("tasks from hooks")

	tasks := resourceTasks
	tasks = append(tasks, hookTasks...)

	// enrich target objects with the namespace
	for _, task := range tasks {
		if task.targetObj == nil {
			continue
		}

		if task.targetObj.GetNamespace() == "" {
			// If target object's namespace is empty, we set namespace in the object. We do
			// this even though it might be a cluster-scoped resource. This prevents any
			// possibility of the resource from unintentionally becoming created in the
			// namespace during the `kubectl apply`
			task.targetObj = task.targetObj.DeepCopy()
			task.targetObj.SetNamespace(sc.namespace)
		}
	}

	// enrich task with live obj
	for _, task := range tasks {
		if task.targetObj == nil || task.liveObj != nil {
			continue
		}
		task.liveObj = sc.liveObj(task.targetObj)
	}

	// enrich tasks with the result
	for _, task := range tasks {
		_, result := sc.syncRes.Resources.Find(task.group(), task.kind(), task.namespace(), task.name(), task.phase)
		if result != nil {
			task.syncStatus = result.Status
			task.operationState = result.HookPhase
			task.message = result.Message
		}
	}

	// check permissions
	for _, task := range tasks {
		serverRes, err := kube.ServerResourceForGroupVersionKind(sc.disco, task.groupVersionKind())
		if err != nil {
			// Special case for custom resources: if CRD is not yet known by the K8s API server,
			// skip verification during `kubectl apply --dry-run` since we expect the CRD
			// to be created during app synchronization.
			if apierr.IsNotFound(err) && sc.hasCRDOfGroupKind(task.group(), task.kind()) {
				sc.log.WithFields(log.Fields{"task": task}).Debug("skip dry-run for custom resource")
				task.skipDryRun = true
			} else {
				sc.setResourceResult(task, v1alpha1.ResultCodeSyncFailed, "", err.Error())
				successful = false
			}
		} else {
			if !sc.proj.IsResourcePermitted(metav1.GroupKind{Group: task.group(), Kind: task.kind()}, serverRes.Namespaced) {
				sc.setResourceResult(task, v1alpha1.ResultCodeSyncFailed, "", fmt.Sprintf("Resource %s:%s is not permitted in project %s.", task.group(), task.kind(), sc.proj.Name))
				successful = false
			}
			if serverRes.Namespaced && !sc.proj.IsDestinationPermitted(v1alpha1.ApplicationDestination{Namespace: task.namespace(), Server: sc.server}) {
				sc.setResourceResult(task, v1alpha1.ResultCodeSyncFailed, "", fmt.Sprintf("namespace %v is not permitted in project '%s'", task.namespace(), sc.proj.Name))
				successful = false
			}
		}
	}

	sort.Sort(tasks)

	return tasks, successful
}

func obj(a, b *unstructured.Unstructured) *unstructured.Unstructured {
	if a != nil {
		return a
	} else {
		return b
	}
}

func (sc *syncContext) liveObj(obj *unstructured.Unstructured) *unstructured.Unstructured {
	for _, resource := range sc.compareResult.managedResources {
		if resource.Group == obj.GroupVersionKind().Group &&
			resource.Kind == obj.GetKind() &&
			// cluster scoped objects will not have a namespace, even if the user has defined it
			(resource.Namespace == "" || resource.Namespace == obj.GetNamespace()) &&
			resource.Name == obj.GetName() {
			return resource.Live
		}
	}
	return nil
}

func (sc *syncContext) setOperationPhase(phase v1alpha1.OperationPhase, message string) {
	if sc.opState.Phase != phase || sc.opState.Message != message {
		sc.log.Infof("Updating operation state. phase: %s -> %s, message: '%s' -> '%s'", sc.opState.Phase, phase, sc.opState.Message, message)
	}
	sc.opState.Phase = phase
	sc.opState.Message = message
}

// ensureCRDReady waits until specified CRD is ready (established condition is true). Method is best effort - it does not fail even if CRD is not ready without timeout.
func (sc *syncContext) ensureCRDReady(name string) {
	_ = wait.PollImmediate(time.Duration(100)*time.Millisecond, crdReadinessTimeout, func() (bool, error) {
		crd, err := sc.extensionsclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, condition := range crd.Status.Conditions {
			if condition.Type == v1beta1.Established {
				return condition.Status == v1beta1.ConditionTrue, nil
			}
		}
		return false, nil
	})
}

// applyObject performs a `kubectl apply` of a single resource
func (sc *syncContext) applyObject(targetObj *unstructured.Unstructured, dryRun bool, force bool) (v1alpha1.ResultCode, string) {
	validate := !resource.HasAnnotationOption(targetObj, common.AnnotationSyncOptions, "Validate=false")
	message, err := sc.kubectl.ApplyResource(sc.config, targetObj, targetObj.GetNamespace(), dryRun, force, validate)
	if err != nil {
		return v1alpha1.ResultCodeSyncFailed, err.Error()
	}
	if kube.IsCRD(targetObj) && !dryRun {
		sc.ensureCRDReady(targetObj.GetName())
	}
	return v1alpha1.ResultCodeSynced, message
}

// pruneObject deletes the object if both prune is true and dryRun is false. Otherwise appropriate message
func (sc *syncContext) pruneObject(liveObj *unstructured.Unstructured, prune, dryRun bool) (v1alpha1.ResultCode, string) {
	if !prune {
		return v1alpha1.ResultCodePruneSkipped, "ignored (requires pruning)"
	} else if resource.HasAnnotationOption(liveObj, common.AnnotationSyncOptions, "Prune=false") {
		return v1alpha1.ResultCodePruneSkipped, "ignored (no prune)"
	} else {
		if dryRun {
			return v1alpha1.ResultCodePruned, "pruned (dry run)"
		} else {
			// Skip deletion if object is already marked for deletion, so we don't cause a resource update hotloop
			deletionTimestamp := liveObj.GetDeletionTimestamp()
			if deletionTimestamp == nil || deletionTimestamp.IsZero() {
				err := sc.kubectl.DeleteResource(sc.config, liveObj.GroupVersionKind(), liveObj.GetName(), liveObj.GetNamespace(), false)
				if err != nil {
					return v1alpha1.ResultCodeSyncFailed, err.Error()
				}
			}
			return v1alpha1.ResultCodePruned, "pruned"
		}
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
	sc.log.Debug("terminating")
	tasks, _ := sc.getSyncTasks()
	for _, task := range tasks {
		if !task.isHook() || !task.completed() {
			continue
		}
		if isRunnable(task.groupVersionKind()) {
			err := sc.deleteResource(task)
			if err != nil {
				sc.setResourceResult(task, "", v1alpha1.OperationFailed, fmt.Sprintf("Failed to delete: %v", err))
				terminateSuccessful = false
			} else {
				sc.setResourceResult(task, "", v1alpha1.OperationSucceeded, fmt.Sprintf("Deleted"))
			}
		}
	}
	if terminateSuccessful {
		sc.setOperationPhase(v1alpha1.OperationFailed, "Operation terminated")
	} else {
		sc.setOperationPhase(v1alpha1.OperationError, "Operation termination had errors")
	}
}

func (sc *syncContext) deleteResource(task *syncTask) error {
	sc.log.WithFields(log.Fields{"task": task}).Debug("deleting task")
	apiResource, err := kube.ServerResourceForGroupVersionKind(sc.disco, task.groupVersionKind())
	if err != nil {
		return err
	}
	resource := kube.ToGroupVersionResource(task.groupVersionKind().GroupVersion().String(), apiResource)
	resIf := kube.ToResourceInterface(sc.dynamicIf, apiResource, resource, task.namespace())
	propagationPolicy := metav1.DeletePropagationForeground
	return resIf.Delete(task.name(), &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}

var operationPhases = map[v1alpha1.ResultCode]v1alpha1.OperationPhase{
	v1alpha1.ResultCodeSynced:       v1alpha1.OperationRunning,
	v1alpha1.ResultCodeSyncFailed:   v1alpha1.OperationFailed,
	v1alpha1.ResultCodePruned:       v1alpha1.OperationSucceeded,
	v1alpha1.ResultCodePruneSkipped: v1alpha1.OperationSucceeded,
}

func (sc *syncContext) runTasks(tasks syncTasks, dryRun bool) bool {

	dryRun = dryRun || sc.syncOp.DryRun

	sc.log.WithFields(log.Fields{"numTasks": len(tasks), "dryRun": dryRun}).Debug("running tasks")

	successful := true
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
			sc.log.WithFields(log.Fields{"dryRun": dryRun, "task": t}).Debug("pruning")
			result, message := sc.pruneObject(t.liveObj, sc.syncOp.Prune, dryRun)
			if result == v1alpha1.ResultCodeSyncFailed {
				successful = false
			}
			if !dryRun || result == v1alpha1.ResultCodeSyncFailed {
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
				sc.log.WithFields(log.Fields{"dryRun": dryRun, "task": t}).Debug("applying")
				result, message := sc.applyObject(t.targetObj, dryRun, sc.syncOp.SyncStrategy.Force())
				if result == v1alpha1.ResultCodeSyncFailed {
					successful = false
				}
				if !dryRun || result == v1alpha1.ResultCodeSyncFailed {
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
func (sc *syncContext) setResourceResult(task *syncTask, syncStatus v1alpha1.ResultCode, operationState v1alpha1.OperationPhase, message string) {

	task.syncStatus = syncStatus
	task.operationState = operationState
	// we always want to keep the latest message
	if message != "" {
		task.message = message
	}

	sc.lock.Lock()
	defer sc.lock.Unlock()
	i, existing := sc.syncRes.Resources.Find(task.group(), task.kind(), task.namespace(), task.name(), task.phase)

	res := v1alpha1.ResourceResult{
		Group:     task.group(),
		Version:   task.version(),
		Kind:      task.kind(),
		Namespace: task.namespace(),
		Name:      task.name(),
		Status:    task.syncStatus,
		Message:   task.message,
		HookType:  task.hookType(),
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
		sc.syncRes.Resources[i] = &res
	} else {
		logCtx.Infof("adding resource result, status: '%s', phase: '%s', message: '%s'", res.Status, res.HookPhase, res.Message)
		sc.syncRes.Resources = append(sc.syncRes.Resources, &res)
	}
}
