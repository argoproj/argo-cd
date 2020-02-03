package sync

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/engine/pkg/utils/health"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	kubeutil "github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/hook"
	"github.com/argoproj/argo-cd/engine/pkg/utils/resource"
	resourceutil "github.com/argoproj/argo-cd/engine/pkg/utils/resource"
)

type reconciledResource struct {
	Target *unstructured.Unstructured
	Live   *unstructured.Unstructured
}

func (r *reconciledResource) key() kube.ResourceKey {
	if r.Live != nil {
		return kube.GetResourceKey(r.Live)
	}
	return kube.GetResourceKey(r.Target)
}

type SyncContext interface {
	Terminate()
	Sync()
	GetState() (common.OperationPhase, string, []common.ResourceSyncResult)
}

type SyncOpt func(ctx *syncContext)

func WithPermissionValidator(validator common.PermissionValidator) SyncOpt {
	return func(ctx *syncContext) {
		ctx.permissionValidator = validator
	}
}

func WithHealthOverride(override health.HealthOverride) SyncOpt {
	return func(ctx *syncContext) {
		ctx.healthOverride = override
	}
}

func WithInitialState(phase common.OperationPhase, message string, results []common.ResourceSyncResult) SyncOpt {
	return func(ctx *syncContext) {
		ctx.phase = phase
		ctx.message = message
		ctx.syncRes = map[string]common.ResourceSyncResult{}
		for i := range results {
			ctx.syncRes[resourceResultKey(results[i].ResourceKey, results[i].SyncPhase)] = results[i]
		}
	}
}

func WithResourcesFilter(resourcesFilter func(key kube.ResourceKey, target *unstructured.Unstructured, live *unstructured.Unstructured) bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.resourcesFilter = resourcesFilter
	}
}

func WithOperationSettings(dryRun bool, prune bool, force bool, skipHooks bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.prune = prune
		ctx.skipHooks = skipHooks
		ctx.dryRun = dryRun
		ctx.force = force
	}
}

func WithManifestValidation(enabled bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.validate = enabled
	}
}

func NewSyncContext(
	revision string,
	reconciliationResult ReconciliationResult,
	restConfig *rest.Config,
	kubectl kubeutil.Kubectl,
	namespace string,
	log *log.Entry,
	opts ...SyncOpt,
) (SyncContext, error) {
	dynamicIf, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	extensionsclientset, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	ctx := &syncContext{
		revision:            revision,
		resources:           groupResources(reconciliationResult),
		hooks:               reconciliationResult.Hooks,
		config:              restConfig,
		dynamicIf:           dynamicIf,
		disco:               disco,
		extensionsclientset: extensionsclientset,
		kubectl:             kubectl,
		namespace:           namespace,
		log:                 log,
		validate:            true,
	}
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx, nil
}

func groupResources(reconciliationResult ReconciliationResult) map[kubeutil.ResourceKey]reconciledResource {
	resources := make(map[kube.ResourceKey]reconciledResource)
	for i := 0; i < len(reconciliationResult.Target); i++ {
		res := reconciledResource{
			Target: reconciliationResult.Target[i],
			Live:   reconciliationResult.Live[i],
		}

		var obj *unstructured.Unstructured
		if res.Live != nil {
			obj = res.Live
		} else {
			obj = res.Target
		}
		resources[kube.GetResourceKey(obj)] = res
	}
	return resources
}

const (
	crdReadinessTimeout = time.Duration(3) * time.Second
)

// getOperationPhase returns a hook status from an _live_ unstructured object
func (sc *syncContext) getOperationPhase(hook *unstructured.Unstructured) (common.OperationPhase, string, error) {
	phase := common.OperationSucceeded
	message := fmt.Sprintf("%s created", hook.GetName())

	resHealth, err := health.GetResourceHealth(hook, sc.healthOverride)
	if err != nil {
		return "", "", err
	}
	if resHealth != nil {
		switch resHealth.Status {
		case health.HealthStatusUnknown, health.HealthStatusDegraded:
			phase = common.OperationFailed
			message = resHealth.Message
		case health.HealthStatusProgressing, health.HealthStatusSuspended:
			phase = common.OperationRunning
			message = resHealth.Message
		case health.HealthStatusHealthy:
			phase = common.OperationSucceeded
			message = resHealth.Message
		}
	}
	return phase, message, nil
}

type syncContext struct {
	healthOverride      health.HealthOverride
	permissionValidator common.PermissionValidator
	resources           map[kube.ResourceKey]reconciledResource
	hooks               []*unstructured.Unstructured
	config              *rest.Config
	dynamicIf           dynamic.Interface
	disco               discovery.DiscoveryInterface
	extensionsclientset *clientset.Clientset
	kubectl             kube.Kubectl
	namespace           string

	dryRun          bool
	force           bool
	validate        bool
	skipHooks       bool
	resourcesFilter func(key kube.ResourceKey, target *unstructured.Unstructured, live *unstructured.Unstructured) bool
	prune           bool

	syncRes   map[string]common.ResourceSyncResult
	startedAt time.Time
	revision  string
	phase     common.OperationPhase
	message   string

	log *log.Entry
	// lock to protect concurrent updates of the result list
	lock sync.Mutex
}

// sync has performs the actual apply or hook based sync
func (sc *syncContext) Sync() {
	sc.log.WithFields(log.Fields{"skipHooks": sc.skipHooks, "started": sc.started()}).Info("syncing")
	tasks, ok := sc.getSyncTasks()
	if !ok {
		sc.setOperationPhase(common.OperationFailed, "one or more synchronization tasks are not valid")
		return
	}

	sc.log.WithFields(log.Fields{"tasks": tasks}).Info("tasks")

	// Perform a `kubectl apply --dry-run` against all the manifests. This will detect most (but
	// not all) validation issues with the user's manifests (e.g. will detect syntax issues, but
	// will not not detect if they are mutating immutable fields). If anything fails, we will refuse
	// to perform the sync. we only wish to do this once per operation, performing additional dry-runs
	// is harmless, but redundant. The indicator we use to detect if we have already performed
	// the dry-run for this operation, is if the resource or hook list is empty.
	if !sc.started() {
		sc.log.Debug("dry-run")
		if sc.runTasks(tasks, true) == failed {
			sc.setOperationPhase(common.OperationFailed, "one or more objects failed to apply (dry run)")
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
			operationState, message, err := sc.getOperationPhase(task.liveObj)
			if err != nil {
				sc.setResourceResult(task, "", common.OperationError, fmt.Sprintf("failed to get resource health: %v", err))
			} else {
				sc.setResourceResult(task, "", operationState, message)

				// maybe delete the hook
				if task.needsDeleting() {
					err := sc.deleteResource(task)
					if err != nil && !errors.IsNotFound(err) {
						sc.setResourceResult(task, "", common.OperationError, fmt.Sprintf("failed to delete resource: %v", err))
					}
				}
			}

		} else {
			// this must be calculated on the live object
			healthStatus, err := health.GetResourceHealth(task.liveObj, sc.healthOverride)
			if err == nil {
				log.WithFields(log.Fields{"task": task, "healthStatus": healthStatus}).Debug("attempting to update health of running task")
				if healthStatus == nil {
					// some objects (e.g. secret) do not have health, and they automatically success
					sc.setResourceResult(task, task.syncStatus, common.OperationSucceeded, task.message)
				} else {
					switch healthStatus.Status {
					case health.HealthStatusHealthy:
						sc.setResourceResult(task, task.syncStatus, common.OperationSucceeded, healthStatus.Message)
					case health.HealthStatusDegraded:
						sc.setResourceResult(task, task.syncStatus, common.OperationFailed, healthStatus.Message)
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
		sc.setOperationPhase(common.OperationRunning, "one or more tasks are running")
		return
	}

	// syncFailTasks only run during failure, so separate them from regular tasks
	syncFailTasks, tasks := tasks.Split(func(t *syncTask) bool { return t.phase == common.SyncPhaseSyncFail })

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
		sc.setOperationPhase(common.OperationSucceeded, "successfully synced (no more tasks)")
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

	sc.setOperationPhase(common.OperationRunning, "one or more tasks are running")

	sc.log.WithFields(log.Fields{"tasks": tasks}).Debug("wet-run")
	runState := sc.runTasks(tasks, false)
	switch runState {
	case failed:
		sc.setOperationFailed(syncFailTasks, "one or more objects failed to apply")
	case successful:
		if complete {
			sc.setOperationPhase(common.OperationSucceeded, "successfully synced (all tasks run)")
		}
	}
}

func (sc *syncContext) GetState() (common.OperationPhase, string, []common.ResourceSyncResult) {
	var resourceRes []common.ResourceSyncResult
	for _, v := range sc.syncRes {
		resourceRes = append(resourceRes, v)
	}
	sort.Slice(resourceRes, func(i, j int) bool {
		return resourceRes[i].Order < resourceRes[j].Order
	})
	return sc.phase, sc.message, resourceRes
}

func (sc *syncContext) setOperationFailed(syncFailTasks syncTasks, message string) {
	if len(syncFailTasks) > 0 {
		// if all the failure hooks are completed, don't run them again, and mark the sync as failed
		if syncFailTasks.All(func(task *syncTask) bool { return task.completed() }) {
			sc.setOperationPhase(common.OperationFailed, message)
			return
		}
		// otherwise, we need to start the failure hooks, and then return without setting
		// the phase, so we make sure we have at least one more sync
		sc.log.WithFields(log.Fields{"syncFailTasks": syncFailTasks}).Debug("running sync fail tasks")
		if sc.runTasks(syncFailTasks, false) == failed {
			sc.setOperationPhase(common.OperationFailed, message)
		}
	} else {
		sc.setOperationPhase(common.OperationFailed, message)
	}
}

func (sc *syncContext) started() bool {
	return len(sc.syncRes) > 0
}

func (sc *syncContext) containsResource(resource reconciledResource) bool {
	return sc.resourcesFilter == nil || sc.resourcesFilter(resource.key(), resource.Live, resource.Target)
}

// generates the list of sync tasks we will be performing during this sync.
func (sc *syncContext) getSyncTasks() (_ syncTasks, successful bool) {
	resourceTasks := syncTasks{}
	successful = true

	for k, resource := range sc.resources {
		if !sc.containsResource(resource) {
			sc.log.WithFields(log.Fields{"group": k.Group, "kind": k.Kind, "name": k.Name}).
				Debug("skipping")
			continue
		}

		obj := obj(resource.Target, resource.Live)

		// this creates garbage tasks
		if hook.IsHook(obj) {
			sc.log.WithFields(log.Fields{"group": obj.GroupVersionKind().Group, "kind": obj.GetKind(), "namespace": obj.GetNamespace(), "name": obj.GetName()}).
				Debug("skipping hook")
			continue
		}

		for _, phase := range syncPhases(obj) {
			resourceTasks = append(resourceTasks, &syncTask{phase: phase, targetObj: resource.Target, liveObj: resource.Live})
		}
	}

	sc.log.WithFields(log.Fields{"resourceTasks": resourceTasks}).Debug("tasks from managed resources")

	hookTasks := syncTasks{}
	if !sc.skipHooks {
		for _, obj := range sc.hooks {
			for _, phase := range syncPhases(obj) {
				// Hook resources names are deterministic, whether they are defined by the user (metadata.name),
				// or formulated at the time of the operation (metadata.generateName). If user specifies
				// metadata.generateName, then we will generate a formulated metadata.name before submission.
				targetObj := obj.DeepCopy()
				if targetObj.GetName() == "" {
					var syncRevision string
					if len(sc.revision) >= 8 {
						syncRevision = sc.revision[0:7]
					} else {
						syncRevision = sc.revision
					}
					postfix := strings.ToLower(fmt.Sprintf("%s-%s-%d", syncRevision, phase, sc.startedAt.UTC().Unix()))
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
		result, ok := sc.syncRes[task.resultKey()]
		if ok {
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
			// and the CRD is part of this sync or the resource is annotated with SkipDryRunOnMissingResource=true,
			// then skip verification during `kubectl apply --dry-run` since we expect the CRD
			// to be created during app synchronization.
			skipDryRunOnMissingResource := resource.HasAnnotationOption(task.targetObj, common.AnnotationSyncOptions, "SkipDryRunOnMissingResource=true")
			if apierr.IsNotFound(err) && (skipDryRunOnMissingResource || sc.hasCRDOfGroupKind(task.group(), task.kind())) {
				sc.log.WithFields(log.Fields{"task": task}).Debug("skip dry-run for custom resource")
				task.skipDryRun = true
			} else {
				sc.setResourceResult(task, common.ResultCodeSyncFailed, "", err.Error())
				successful = false
			}
		} else {
			if err := sc.permissionValidator(task.obj(), serverRes); err != nil {
				sc.setResourceResult(task, common.ResultCodeSyncFailed, "", err.Error())
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
	for k, resource := range sc.resources {
		if k.Group == obj.GroupVersionKind().Group &&
			k.Kind == obj.GetKind() &&
			// cluster scoped objects will not have a namespace, even if the user has defined it
			(k.Namespace == "" || k.Namespace == obj.GetNamespace()) &&
			k.Name == obj.GetName() {
			return resource.Live
		}
	}
	return nil
}

func (sc *syncContext) setOperationPhase(phase common.OperationPhase, message string) {
	if sc.phase != phase || sc.message != message {
		sc.log.Infof("Updating operation state. phase: %s -> %s, message: '%s' -> '%s'", sc.phase, phase, sc.message, message)
	}
	sc.phase = phase
	sc.message = message
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

func (sc *syncContext) applyObject(targetObj *unstructured.Unstructured, dryRun bool, force bool, validate bool) (common.ResultCode, string) {
	message, err := sc.kubectl.ApplyResource(sc.config, targetObj, targetObj.GetNamespace(), dryRun, force, validate)
	if err != nil {
		return common.ResultCodeSyncFailed, err.Error()
	}
	if kube.IsCRD(targetObj) && !dryRun {
		sc.ensureCRDReady(targetObj.GetName())
	}
	return common.ResultCodeSynced, message
}

// pruneObject deletes the object if both prune is true and dryRun is false. Otherwise appropriate message
func (sc *syncContext) pruneObject(liveObj *unstructured.Unstructured, prune, dryRun bool) (common.ResultCode, string) {
	if !prune {
		return common.ResultCodePruneSkipped, "ignored (requires pruning)"
	} else if resourceutil.HasAnnotationOption(liveObj, common.AnnotationSyncOptions, "Prune=false") {
		return common.ResultCodePruneSkipped, "ignored (no prune)"
	} else {
		if dryRun {
			return common.ResultCodePruned, "pruned (dry run)"
		} else {
			// Skip deletion if object is already marked for deletion, so we don't cause a resource update hotloop
			deletionTimestamp := liveObj.GetDeletionTimestamp()
			if deletionTimestamp == nil || deletionTimestamp.IsZero() {
				err := sc.kubectl.DeleteResource(sc.config, liveObj.GroupVersionKind(), liveObj.GetName(), liveObj.GetNamespace(), false)
				if err != nil {
					return common.ResultCodeSyncFailed, err.Error()
				}
			}
			return common.ResultCodePruned, "pruned"
		}
	}
}

func (sc *syncContext) targetObjs() []*unstructured.Unstructured {
	objs := sc.hooks
	for _, r := range sc.resources {
		if r.Target != nil {
			objs = append(objs, r.Target)
		}
	}
	return objs
}

func (sc *syncContext) hasCRDOfGroupKind(group string, kind string) bool {
	for _, obj := range sc.targetObjs() {
		if kube.IsCRD(obj) {
			crdGroup, ok, err := unstructured.NestedString(obj.Object, "spec", "group")
			if err != nil || !ok {
				continue
			}
			crdKind, ok, err := unstructured.NestedString(obj.Object, "spec", "names", "kind")
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
func (sc *syncContext) Terminate() {
	terminateSuccessful := true
	sc.log.Debug("terminating")
	tasks, _ := sc.getSyncTasks()
	for _, task := range tasks {
		if !task.isHook() || task.liveObj == nil {
			continue
		}
		phase, msg, err := sc.getOperationPhase(task.liveObj)
		if err != nil {
			sc.setOperationPhase(common.OperationError, fmt.Sprintf("Failed to get hook health: %v", err))
			return
		}
		if phase == common.OperationRunning {
			err := sc.deleteResource(task)
			if err != nil {
				sc.setResourceResult(task, "", common.OperationFailed, fmt.Sprintf("Failed to delete: %v", err))
				terminateSuccessful = false
			} else {
				sc.setResourceResult(task, "", common.OperationSucceeded, fmt.Sprintf("Deleted"))
			}
		} else {
			sc.setResourceResult(task, "", phase, msg)
		}
	}
	if terminateSuccessful {
		sc.setOperationPhase(common.OperationFailed, "Operation terminated")
	} else {
		sc.setOperationPhase(common.OperationError, "Operation termination had errors")
	}
}

func (sc *syncContext) deleteResource(task *syncTask) error {
	sc.log.WithFields(log.Fields{"task": task}).Debug("deleting resource")
	resIf, err := sc.getResourceIf(task)
	if err != nil {
		return err
	}
	propagationPolicy := metav1.DeletePropagationForeground
	return resIf.Delete(task.name(), &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}

func (sc *syncContext) getResourceIf(task *syncTask) (dynamic.ResourceInterface, error) {
	apiResource, err := kube.ServerResourceForGroupVersionKind(sc.disco, task.groupVersionKind())
	if err != nil {
		return nil, err
	}
	res := kube.ToGroupVersionResource(task.groupVersionKind().GroupVersion().String(), apiResource)
	resIf := kube.ToResourceInterface(sc.dynamicIf, apiResource, res, task.namespace())
	return resIf, err
}

var operationPhases = map[common.ResultCode]common.OperationPhase{
	common.ResultCodeSynced:       common.OperationRunning,
	common.ResultCodeSyncFailed:   common.OperationFailed,
	common.ResultCodePruned:       common.OperationSucceeded,
	common.ResultCodePruneSkipped: common.OperationSucceeded,
}

// tri-state
type runState = int

const (
	successful = iota
	pending
	failed
)

func (sc *syncContext) runTasks(tasks syncTasks, dryRun bool) runState {

	dryRun = dryRun || sc.dryRun

	sc.log.WithFields(log.Fields{"numTasks": len(tasks), "dryRun": dryRun}).Debug("running tasks")

	runState := successful
	var createTasks syncTasks
	var pruneTasks syncTasks

	for _, task := range tasks {
		if task.isPrune() {
			pruneTasks = append(pruneTasks, task)
		} else {
			createTasks = append(createTasks, task)
		}
	}
	// prune first
	{
		var wg sync.WaitGroup
		for _, task := range pruneTasks {
			wg.Add(1)
			go func(t *syncTask) {
				defer wg.Done()
				logCtx := sc.log.WithFields(log.Fields{"dryRun": dryRun, "task": t})
				logCtx.Debug("pruning")
				result, message := sc.pruneObject(t.liveObj, sc.prune, dryRun)
				if result == common.ResultCodeSyncFailed {
					runState = failed
					logCtx.WithField("message", message).Info("pruning failed")
				}
				if !dryRun || sc.dryRun || result == common.ResultCodeSyncFailed {
					sc.setResourceResult(t, result, operationPhases[result], message)
				}
			}(task)
		}
		wg.Wait()
	}

	// delete anything that need deleting
	if runState == successful && createTasks.Any(func(t *syncTask) bool { return t.needsDeleting() }) {
		var wg sync.WaitGroup
		for _, task := range createTasks.Filter(func(t *syncTask) bool { return t.needsDeleting() }) {
			wg.Add(1)
			go func(t *syncTask) {
				defer wg.Done()
				sc.log.WithFields(log.Fields{"dryRun": dryRun, "task": t}).Debug("deleting")
				if !dryRun {
					err := sc.deleteResource(t)
					if err != nil {
						// it is possible to get a race condition here, such that the resource does not exist when
						// delete is requested, we treat this as a nop
						if !apierr.IsNotFound(err) {
							runState = failed
							sc.setResourceResult(t, "", common.OperationError, fmt.Sprintf("failed to delete resource: %v", err))
						}
					} else {
						// if there is anything that needs deleting, we are at best now in pending and
						// want to return and wait for sync to be invoked again
						runState = pending
					}
				}
			}(task)
		}
		wg.Wait()
	}
	// finally create resources
	if runState == successful {
		processCreateTasks := func(tasks syncTasks) {
			var createWg sync.WaitGroup
			for _, task := range tasks {
				if dryRun && task.skipDryRun {
					continue
				}
				createWg.Add(1)
				go func(t *syncTask) {
					defer createWg.Done()
					logCtx := sc.log.WithFields(log.Fields{"dryRun": dryRun, "task": t})
					logCtx.Debug("applying")
					validate := sc.validate && !resourceutil.HasAnnotationOption(t.targetObj, common.AnnotationSyncOptions, "Validate=false")
					result, message := sc.applyObject(t.targetObj, dryRun, sc.force, validate)
					if result == common.ResultCodeSyncFailed {
						logCtx.WithField("message", message).Info("apply failed")
						runState = failed
					}
					if !dryRun || sc.dryRun || result == common.ResultCodeSyncFailed {
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
	}
	return runState
}

// setResourceResult sets a resource details in the SyncResult.Resources list
func (sc *syncContext) setResourceResult(task *syncTask, syncStatus common.ResultCode, operationState common.OperationPhase, message string) {
	task.syncStatus = syncStatus
	task.operationState = operationState
	// we always want to keep the latest message
	if message != "" {
		task.message = message
	}

	sc.lock.Lock()
	defer sc.lock.Unlock()
	existing, ok := sc.syncRes[task.resultKey()]

	res := common.ResourceSyncResult{
		ResourceKey: kube.GetResourceKey(task.obj()),
		Version:     task.version(),
		Status:      task.syncStatus,
		Message:     task.message,
		HookType:    task.hookType(),
		HookPhase:   task.operationState,
		SyncPhase:   task.phase,
	}

	logCtx := sc.log.WithFields(log.Fields{"namespace": task.namespace(), "kind": task.kind(), "name": task.name(), "phase": task.phase})

	if ok {
		// update existing value
		if res.Status != existing.Status || res.HookPhase != existing.HookPhase || res.Message != existing.Message {
			logCtx.Infof("updating resource result, status: '%s' -> '%s', phase '%s' -> '%s', message '%s' -> '%s'",
				existing.Status, res.Status,
				existing.HookPhase, res.HookPhase,
				existing.Message, res.Message)
			existing.Status = res.Status
			existing.HookPhase = res.HookPhase
			existing.Message = res.Message
		}
		sc.syncRes[task.resultKey()] = existing
	} else {
		logCtx.Infof("adding resource result, status: '%s', phase: '%s', message: '%s'", res.Status, res.HookPhase, res.Message)
		res.Order = len(sc.syncRes) + 1
		sc.syncRes[task.resultKey()] = res
	}
}

func resourceResultKey(key kubeutil.ResourceKey, phase common.SyncPhase) string {
	return fmt.Sprintf("%s:%s", key.String(), phase)
}
