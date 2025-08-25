package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2/textlogger"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/sync/hook"
	resourceutil "github.com/argoproj/gitops-engine/pkg/sync/resource"
	kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
)

type reconciledResource struct {
	Target *unstructured.Unstructured
	Live   *unstructured.Unstructured
}

func (r *reconciledResource) key() kubeutil.ResourceKey {
	if r.Live != nil {
		return kubeutil.GetResourceKey(r.Live)
	}
	return kubeutil.GetResourceKey(r.Target)
}

// SyncContext defines an interface that allows to execute sync operation step or terminate it.
type SyncContext interface {
	// Terminate terminates sync operation. The method is asynchronous: it starts deletion is related K8S resources
	// such as in-flight resource hooks, updates operation status, and exists without waiting for resource completion.
	Terminate()
	// Executes next synchronization step and updates operation status.
	Sync()
	// Returns current sync operation state and information about resources synchronized so far.
	GetState() (common.OperationPhase, string, []common.ResourceSyncResult)
}

// SyncOpt is a callback that update sync operation settings
type SyncOpt func(ctx *syncContext)

// WithPrunePropagationPolicy sets specified permission validator
func WithPrunePropagationPolicy(policy *metav1.DeletionPropagation) SyncOpt {
	return func(ctx *syncContext) {
		ctx.prunePropagationPolicy = policy
	}
}

// WithPermissionValidator sets specified permission validator
func WithPermissionValidator(validator common.PermissionValidator) SyncOpt {
	return func(ctx *syncContext) {
		ctx.permissionValidator = validator
	}
}

// WithHealthOverride sets specified health override
func WithHealthOverride(override health.HealthOverride) SyncOpt {
	return func(ctx *syncContext) {
		ctx.healthOverride = override
	}
}

// WithInitialState sets sync operation initial state
func WithInitialState(phase common.OperationPhase, message string, results []common.ResourceSyncResult, startedAt metav1.Time) SyncOpt {
	return func(ctx *syncContext) {
		ctx.phase = phase
		ctx.message = message
		ctx.syncRes = map[string]common.ResourceSyncResult{}
		ctx.startedAt = startedAt.Time
		for i := range results {
			ctx.syncRes[resourceResultKey(results[i].ResourceKey, results[i].SyncPhase)] = results[i]
		}
	}
}

// WithResourcesFilter sets sync operation resources filter
func WithResourcesFilter(resourcesFilter func(key kubeutil.ResourceKey, target *unstructured.Unstructured, live *unstructured.Unstructured) bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.resourcesFilter = resourcesFilter
	}
}

// WithSkipHooks specifies if hooks should be enabled or not
func WithSkipHooks(skipHooks bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.skipHooks = skipHooks
	}
}

// WithPrune specifies if resource pruning enabled
func WithPrune(prune bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.prune = prune
	}
}

// WithPruneConfirmed specifies if prune is confirmed for resources that require confirmation
func WithPruneConfirmed(confirmed bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.pruneConfirmed = confirmed
	}
}

// WithOperationSettings allows to set sync operation settings
func WithOperationSettings(dryRun bool, prune bool, force bool, skipHooks bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.prune = prune
		ctx.skipHooks = skipHooks
		ctx.dryRun = dryRun
		ctx.force = force
	}
}

// WithManifestValidation enables or disables manifest validation
func WithManifestValidation(enabled bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.validate = enabled
	}
}

// WithPruneLast enables or disables pruneLast
func WithPruneLast(enabled bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.pruneLast = enabled
	}
}

// WithResourceModificationChecker sets resource modification result
func WithResourceModificationChecker(enabled bool, diffResults *diff.DiffResultList) SyncOpt {
	return func(ctx *syncContext) {
		ctx.applyOutOfSyncOnly = enabled
		if enabled {
			ctx.modificationResult = groupDiffResults(diffResults)
		} else {
			ctx.modificationResult = nil
		}
	}
}

// WithNamespaceModifier will create a namespace with the metadata passed in the `*unstructured.Unstructured` argument
// of the `namespaceModifier` function, in the case it returns `true`. If the namespace already exists, the metadata
// will overwrite what is already present if `namespaceModifier` returns `true`. If `namespaceModifier` returns `false`,
// this will be a no-op.
func WithNamespaceModifier(namespaceModifier func(*unstructured.Unstructured, *unstructured.Unstructured) (bool, error)) SyncOpt {
	return func(ctx *syncContext) {
		ctx.syncNamespace = namespaceModifier
	}
}

// WithLogr sets the logger to use.
func WithLogr(log logr.Logger) SyncOpt {
	return func(ctx *syncContext) {
		ctx.log = log
	}
}

// WithSyncWaveHook sets a callback that is invoked after application of every wave
func WithSyncWaveHook(syncWaveHook common.SyncWaveHook) SyncOpt {
	return func(ctx *syncContext) {
		ctx.syncWaveHook = syncWaveHook
	}
}

func WithReplace(replace bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.replace = replace
	}
}

func WithSkipDryRunOnMissingResource(skipDryRunOnMissingResource bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.skipDryRunOnMissingResource = skipDryRunOnMissingResource
	}
}

func WithServerSideApply(serverSideApply bool) SyncOpt {
	return func(ctx *syncContext) {
		ctx.serverSideApply = serverSideApply
	}
}

func WithServerSideApplyManager(manager string) SyncOpt {
	return func(ctx *syncContext) {
		ctx.serverSideApplyManager = manager
	}
}

// WithClientSideApplyMigration configures client-side apply migration for server-side apply.
// When enabled, fields managed by the specified manager will be migrated to server-side apply.
// Defaults to enabled=true with manager="kubectl-client-side-apply" if not configured.
func WithClientSideApplyMigration(enabled bool, manager string) SyncOpt {
	return func(ctx *syncContext) {
		ctx.enableClientSideApplyMigration = enabled
		if enabled && manager != "" {
			ctx.clientSideApplyMigrationManager = manager
		}
	}
}

// NewSyncContext creates new instance of a SyncContext
func NewSyncContext(
	revision string,
	reconciliationResult ReconciliationResult,
	restConfig *rest.Config,
	rawConfig *rest.Config,
	kubectl kubeutil.Kubectl,
	namespace string,
	openAPISchema openapi.Resources,
	opts ...SyncOpt,
) (SyncContext, func(), error) {
	dynamicIf, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create discovery client: %w", err)
	}
	extensionsclientset, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create extensions client: %w", err)
	}
	resourceOps, cleanup, err := kubectl.ManageResources(rawConfig, openAPISchema)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to manage resources: %w", err)
	}
	ctx := &syncContext{
		revision:                        revision,
		resources:                       groupResources(reconciliationResult),
		hooks:                           reconciliationResult.Hooks,
		config:                          restConfig,
		rawConfig:                       rawConfig,
		dynamicIf:                       dynamicIf,
		disco:                           disco,
		extensionsclientset:             extensionsclientset,
		kubectl:                         kubectl,
		resourceOps:                     resourceOps,
		namespace:                       namespace,
		log:                             textlogger.NewLogger(textlogger.NewConfig()),
		validate:                        true,
		startedAt:                       time.Now(),
		syncRes:                         map[string]common.ResourceSyncResult{},
		clientSideApplyMigrationManager: common.DefaultClientSideApplyMigrationManager,
		enableClientSideApplyMigration:  true,
		permissionValidator: func(_ *unstructured.Unstructured, _ *metav1.APIResource) error {
			return nil
		},
	}
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx, cleanup, nil
}

func groupResources(reconciliationResult ReconciliationResult) map[kubeutil.ResourceKey]reconciledResource {
	resources := make(map[kubeutil.ResourceKey]reconciledResource)
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
		resources[kubeutil.GetResourceKey(obj)] = res
	}
	return resources
}

// generates a map of resource and its modification result based on diffResultList
func groupDiffResults(diffResultList *diff.DiffResultList) map[kubeutil.ResourceKey]bool {
	modifiedResources := make(map[kubeutil.ResourceKey]bool)
	for _, res := range diffResultList.Diffs {
		var obj unstructured.Unstructured
		var err error
		if string(res.NormalizedLive) != "null" {
			err = json.Unmarshal(res.NormalizedLive, &obj)
		} else {
			err = json.Unmarshal(res.PredictedLive, &obj)
		}
		if err != nil {
			continue
		}
		modifiedResources[kubeutil.GetResourceKey(&obj)] = res.Modified
	}
	return modifiedResources
}

const (
	crdReadinessTimeout = time.Duration(3) * time.Second
)

// getOperationPhase returns a health status from a _live_ unstructured object
func (sc *syncContext) getOperationPhase(obj *unstructured.Unstructured) (common.OperationPhase, string, error) {
	phase := common.OperationSucceeded
	message := obj.GetName() + " created"

	resHealth, err := health.GetResourceHealth(obj, sc.healthOverride)
	if err != nil {
		return "", "", fmt.Errorf("failed to get resource health: %w", err)
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
	resources           map[kubeutil.ResourceKey]reconciledResource
	hooks               []*unstructured.Unstructured
	config              *rest.Config
	rawConfig           *rest.Config
	dynamicIf           dynamic.Interface
	disco               discovery.DiscoveryInterface
	extensionsclientset *clientset.Clientset
	kubectl             kubeutil.Kubectl
	resourceOps         kubeutil.ResourceOperations
	namespace           string

	dryRun                          bool
	skipDryRunOnMissingResource     bool
	force                           bool
	validate                        bool
	skipHooks                       bool
	resourcesFilter                 func(key kubeutil.ResourceKey, target *unstructured.Unstructured, live *unstructured.Unstructured) bool
	prune                           bool
	replace                         bool
	serverSideApply                 bool
	serverSideApplyManager          string
	pruneLast                       bool
	prunePropagationPolicy          *metav1.DeletionPropagation
	pruneConfirmed                  bool
	clientSideApplyMigrationManager string
	enableClientSideApplyMigration  bool

	syncRes   map[string]common.ResourceSyncResult
	startedAt time.Time
	revision  string
	phase     common.OperationPhase
	message   string

	log logr.Logger
	// lock to protect concurrent updates of the result list
	lock sync.Mutex

	// syncNamespace is a function that will determine if the managed
	// namespace should be synced
	syncNamespace func(*unstructured.Unstructured, *unstructured.Unstructured) (bool, error)

	syncWaveHook common.SyncWaveHook

	applyOutOfSyncOnly bool
	// stores whether the resource is modified or not
	modificationResult map[kubeutil.ResourceKey]bool
}

func (sc *syncContext) setRunningPhase(tasks []*syncTask, isPendingDeletion bool) {
	if len(tasks) > 0 {
		firstTask := tasks[0]
		waitingFor := "completion of hook"
		andMore := "hooks"
		if !firstTask.isHook() {
			waitingFor = "healthy state of"
			andMore = "resources"
		}
		if isPendingDeletion {
			waitingFor = "deletion of"
		}
		message := fmt.Sprintf("waiting for %s %s/%s/%s",
			waitingFor, firstTask.group(), firstTask.kind(), firstTask.name())
		if moreTasks := len(tasks) - 1; moreTasks > 0 {
			message = fmt.Sprintf("%s and %d more %s", message, moreTasks, andMore)
		}
		sc.setOperationPhase(common.OperationRunning, message)
	}
}

// sync has performs the actual apply or hook based sync
func (sc *syncContext) Sync() {
	sc.log.WithValues("skipHooks", sc.skipHooks, "started", sc.started()).Info("Syncing")
	tasks, ok := sc.getSyncTasks()
	if !ok {
		sc.setOperationPhase(common.OperationFailed, "one or more synchronization tasks are not valid")
		return
	}

	if sc.started() {
		sc.log.WithValues("tasks", tasks).Info("Tasks")
	} else {
		// Perform a `kubectl apply --dry-run` against all the manifests. This will detect most (but
		// not all) validation issues with the user's manifests (e.g. will detect syntax issues, but
		// will not not detect if they are mutating immutable fields). If anything fails, we will refuse
		// to perform the sync. we only wish to do this once per operation, performing additional dry-runs
		// is harmless, but redundant. The indicator we use to detect if we have already performed
		// the dry-run for this operation, is if the resource or hook list is empty.
		dryRunTasks := tasks

		// Before doing any validation, we have to create the application namespace if it does not exist.
		// The validation is expected to fail in multiple scenarios if a namespace does not exist.
		if nsCreateTask := sc.getNamespaceCreationTask(dryRunTasks); nsCreateTask != nil {
			nsSyncTasks := syncTasks{nsCreateTask}
			// No need to perform a dry-run on the namespace creation, because if it fails we stop anyway
			sc.log.WithValues("task", nsCreateTask).Info("Creating namespace")
			if sc.runTasks(nsSyncTasks, false) == failed {
				sc.setOperationFailed(syncTasks{}, nsSyncTasks, "the namespace failed to apply")
				return
			}

			// The namespace was created, we can remove this task from the dry-run
			dryRunTasks = tasks.Filter(func(t *syncTask) bool { return t != nsCreateTask })
		}

		if sc.applyOutOfSyncOnly {
			dryRunTasks = sc.filterOutOfSyncTasks(dryRunTasks)
		}

		sc.log.WithValues("tasks", dryRunTasks).Info("Tasks (dry-run)")
		if sc.runTasks(dryRunTasks, true) == failed {
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
			}
		} else {
			// this must be calculated on the live object
			healthStatus, err := health.GetResourceHealth(task.liveObj, sc.healthOverride)
			if err == nil {
				sc.log.WithValues("task", task, "healthStatus", healthStatus).V(1).Info("attempting to update health of running task")
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
	runningTasks := tasks.Filter(func(t *syncTask) bool { return (multiStep || t.isHook()) && t.running() })
	if runningTasks.Len() > 0 {
		sc.setRunningPhase(runningTasks, false)
		return
	}

	// if pruned tasks pending deletion, then wait...
	prunedTasksPendingDelete := tasks.Filter(func(t *syncTask) bool {
		if t.pruned() && t.liveObj != nil {
			return t.liveObj.GetDeletionTimestamp() != nil
		}
		return false
	})
	if prunedTasksPendingDelete.Len() > 0 {
		sc.setRunningPhase(prunedTasksPendingDelete, true)
		return
	}

	hooksCompleted := tasks.Filter(func(task *syncTask) bool {
		return task.isHook() && task.completed()
	})
	for _, task := range hooksCompleted {
		if err := sc.removeHookFinalizer(task); err != nil {
			sc.setResourceResult(task, task.syncStatus, common.OperationError, fmt.Sprintf("Failed to remove hook finalizer: %v", err))
		}
	}

	// collect all completed hooks which have appropriate delete policy
	hooksPendingDeletionSuccessful := tasks.Filter(func(task *syncTask) bool {
		return task.isHook() && task.liveObj != nil && !task.running() && task.deleteOnPhaseSuccessful()
	})

	hooksPendingDeletionFailed := tasks.Filter(func(task *syncTask) bool {
		return task.isHook() && task.liveObj != nil && !task.running() && task.deleteOnPhaseFailed()
	})

	// syncFailTasks only run during failure, so separate them from regular tasks
	syncFailTasks, tasks := tasks.Split(func(t *syncTask) bool { return t.phase == common.SyncPhaseSyncFail })

	syncFailedTasks, _ := tasks.Split(func(t *syncTask) bool { return t.syncStatus == common.ResultCodeSyncFailed })

	// if there are any completed but unsuccessful tasks, sync is a failure.
	if tasks.Any(func(t *syncTask) bool { return t.completed() && !t.successful() }) {
		sc.deleteHooks(hooksPendingDeletionFailed)
		sc.setOperationFailed(syncFailTasks, syncFailedTasks, "one or more synchronization tasks completed unsuccessfully")
		return
	}

	sc.log.WithValues("tasks", tasks).V(1).Info("Filtering out non-pending tasks")
	// remove tasks that are completed, we can assume that there are no running tasks
	tasks = tasks.Filter(func(t *syncTask) bool { return t.pending() })

	if sc.applyOutOfSyncOnly {
		tasks = sc.filterOutOfSyncTasks(tasks)
	}

	// If no sync tasks were generated (e.g., in case all application manifests have been removed),
	// the sync operation is successful.
	if len(tasks) == 0 {
		// delete all completed hooks which have appropriate delete policy
		sc.deleteHooks(hooksPendingDeletionSuccessful)
		sc.setOperationPhase(common.OperationSucceeded, "successfully synced (no more tasks)")
		return
	}

	// remove any tasks not in this wave
	phase := tasks.phase()
	wave := tasks.wave()
	finalWave := phase == tasks.lastPhase() && wave == tasks.lastWave()

	// if it is the last phase/wave and the only remaining tasks are non-hooks, the we are successful
	// EVEN if those objects subsequently degraded
	// This handles the common case where neither hooks or waves are used and a sync equates to simply an (asynchronous) kubectl apply of manifests, which succeeds immediately.
	remainingTasks := tasks.Filter(func(t *syncTask) bool { return t.phase != phase || wave != t.wave() || t.isHook() })

	sc.log.WithValues("phase", phase, "wave", wave, "tasks", tasks, "syncFailTasks", syncFailTasks).V(1).Info("Filtering tasks in correct phase and wave")
	tasks = tasks.Filter(func(t *syncTask) bool { return t.phase == phase && t.wave() == wave })

	sc.setOperationPhase(common.OperationRunning, "one or more tasks are running")

	sc.log.WithValues("tasks", tasks).V(1).Info("Wet-run")
	runState := sc.runTasks(tasks, false)

	if sc.syncWaveHook != nil && runState != failed {
		err := sc.syncWaveHook(phase, wave, finalWave)
		if err != nil {
			sc.deleteHooks(hooksPendingDeletionFailed)
			sc.setOperationPhase(common.OperationFailed, fmt.Sprintf("SyncWaveHook failed: %v", err))
			sc.log.Error(err, "SyncWaveHook failed")
			return
		}
	}

	switch runState {
	case failed:
		syncFailedTasks, _ := tasks.Split(func(t *syncTask) bool { return t.syncStatus == common.ResultCodeSyncFailed })
		sc.deleteHooks(hooksPendingDeletionFailed)
		sc.setOperationFailed(syncFailTasks, syncFailedTasks, "one or more objects failed to apply")
	case successful:
		if remainingTasks.Len() == 0 {
			// delete all completed hooks which have appropriate delete policy
			sc.deleteHooks(hooksPendingDeletionSuccessful)
			sc.setOperationPhase(common.OperationSucceeded, "successfully synced (all tasks run)")
		} else {
			sc.setRunningPhase(remainingTasks, false)
		}
	default:
		sc.setRunningPhase(tasks.Filter(func(task *syncTask) bool {
			return task.deleteOnPhaseCompletion()
		}), true)
	}
}

// filter out out-of-sync tasks
func (sc *syncContext) filterOutOfSyncTasks(tasks syncTasks) syncTasks {
	return tasks.Filter(func(t *syncTask) bool {
		if t.isHook() {
			return true
		}

		if modified, ok := sc.modificationResult[t.resourceKey()]; !modified && ok && t.targetObj != nil && t.liveObj != nil {
			sc.log.WithValues("resource key", t.resourceKey()).V(1).Info("Skipping as resource was not modified")
			return false
		}
		return true
	})
}

// getNamespaceCreationTask returns a task that will create the current namespace
// or nil if the syncTasks does not contain one
func (sc *syncContext) getNamespaceCreationTask(tasks syncTasks) *syncTask {
	creationTasks := tasks.Filter(func(task *syncTask) bool {
		return task.liveObj == nil && isNamespaceWithName(task.targetObj, sc.namespace)
	})
	if len(creationTasks) > 0 {
		return creationTasks[0]
	}
	return nil
}

func (sc *syncContext) removeHookFinalizer(task *syncTask) error {
	if task.liveObj == nil {
		return nil
	}
	removeFinalizerMutation := func(obj *unstructured.Unstructured) bool {
		finalizers := obj.GetFinalizers()
		for i, finalizer := range finalizers {
			if finalizer == hook.HookFinalizer {
				obj.SetFinalizers(append(finalizers[:i], finalizers[i+1:]...))
				return true
			}
		}
		return false
	}

	// The cached live object may be stale in the controller cache, and the actual object may have been updated in the meantime,
	// and Kubernetes API will return a conflict error on the Update call.
	// In that case, we need to get the latest version of the object and retry the update.
	//nolint:wrapcheck // wrap inside the retried function instead
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		mutated := removeFinalizerMutation(task.liveObj)
		if !mutated {
			return nil
		}

		updateErr := sc.updateResource(task)
		if apierrors.IsConflict(updateErr) {
			sc.log.WithValues("task", task).V(1).Info("Retrying hook finalizer removal due to conflict on update")
			resIf, err := sc.getResourceIf(task, "get")
			if err != nil {
				return fmt.Errorf("failed to get resource interface: %w", err)
			}
			liveObj, err := resIf.Get(context.TODO(), task.liveObj.GetName(), metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				sc.log.WithValues("task", task).V(1).Info("Resource is already deleted")
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to get resource: %w", err)
			}
			task.liveObj = liveObj
		} else if apierrors.IsNotFound(updateErr) {
			// If the resource is already deleted, it is a no-op
			sc.log.WithValues("task", task).V(1).Info("Resource is already deleted")
			return nil
		}
		if updateErr != nil {
			return fmt.Errorf("failed to update resource: %w", updateErr)
		}
		return nil
	})
}

func (sc *syncContext) updateResource(task *syncTask) error {
	sc.log.WithValues("task", task).V(1).Info("Updating resource")
	resIf, err := sc.getResourceIf(task, "update")
	if err != nil {
		return err
	}
	_, err = resIf.Update(context.TODO(), task.liveObj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}
	return nil
}

func (sc *syncContext) deleteHooks(hooksPendingDeletion syncTasks) {
	for _, task := range hooksPendingDeletion {
		err := sc.deleteResource(task)
		if err != nil && !apierrors.IsNotFound(err) {
			sc.setResourceResult(task, "", common.OperationError, fmt.Sprintf("failed to delete resource: %v", err))
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

func (sc *syncContext) setOperationFailed(syncFailTasks, syncFailedTasks syncTasks, message string) {
	errorMessageFactory := func(tasks syncTasks, message string) string {
		messages := tasks.Map(func(task *syncTask) string {
			return task.message
		})
		if len(messages) > 0 {
			return fmt.Sprintf("%s, reason: %s", message, strings.Join(messages, ","))
		}
		return message
	}

	errorMessage := errorMessageFactory(syncFailedTasks, message)

	if len(syncFailTasks) > 0 {
		// if all the failure hooks are completed, don't run them again, and mark the sync as failed
		if syncFailTasks.All(func(task *syncTask) bool { return task.completed() }) {
			sc.setOperationPhase(common.OperationFailed, errorMessage)
			return
		}
		// otherwise, we need to start the failure hooks, and then return without setting
		// the phase, so we make sure we have at least one more sync
		sc.log.WithValues("syncFailTasks", syncFailTasks).V(1).Info("Running sync fail tasks")
		if sc.runTasks(syncFailTasks, false) == failed {
			failedSyncFailTasks := syncFailTasks.Filter(func(t *syncTask) bool { return t.syncStatus == common.ResultCodeSyncFailed })
			syncFailTasksMessage := errorMessageFactory(failedSyncFailTasks, "one or more SyncFail hooks failed")
			sc.setOperationPhase(common.OperationFailed, fmt.Sprintf("%s\n%s", errorMessage, syncFailTasksMessage))
		}
	} else {
		sc.setOperationPhase(common.OperationFailed, errorMessage)
	}
}

func (sc *syncContext) started() bool {
	return len(sc.syncRes) > 0
}

func (sc *syncContext) containsResource(resource reconciledResource) bool {
	return sc.resourcesFilter == nil || sc.resourcesFilter(resource.key(), resource.Target, resource.Live)
}

// generates the list of sync tasks we will be performing during this sync.
func (sc *syncContext) getSyncTasks() (_ syncTasks, successful bool) {
	resourceTasks := syncTasks{}
	successful = true

	for k, resource := range sc.resources {
		if !sc.containsResource(resource) {
			sc.log.WithValues("group", k.Group, "kind", k.Kind, "name", k.Name).V(1).Info("Skipping")
			continue
		}

		obj := obj(resource.Target, resource.Live)

		// this creates garbage tasks
		if hook.IsHook(obj) {
			sc.log.WithValues("group", obj.GroupVersionKind().Group, "kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName()).V(1).Info("Skipping hook")
			continue
		}

		for _, phase := range syncPhases(obj) {
			resourceTasks = append(resourceTasks, &syncTask{phase: phase, targetObj: resource.Target, liveObj: resource.Live})
		}
	}

	sc.log.WithValues("resourceTasks", resourceTasks).V(1).Info("Tasks from managed resources")

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
				if !hook.HasHookFinalizer(targetObj) {
					targetObj.SetFinalizers(append(targetObj.GetFinalizers(), hook.HookFinalizer))
				}
				hookTasks = append(hookTasks, &syncTask{phase: phase, targetObj: targetObj})
			}
		}
	}

	sc.log.WithValues("hookTasks", hookTasks).V(1).Info("tasks from hooks")

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

	if sc.syncNamespace != nil && sc.namespace != "" {
		tasks = sc.autoCreateNamespace(tasks)
	}

	// enrich task with live obj
	for _, task := range tasks {
		if task.targetObj == nil || task.liveObj != nil {
			continue
		}
		task.liveObj = sc.liveObj(task.targetObj)
	}

	isRetryable := apierrors.IsUnauthorized

	serverResCache := make(map[schema.GroupVersionKind]*metav1.APIResource)

	// check permissions
	for _, task := range tasks {
		var serverRes *metav1.APIResource
		var err error

		if val, ok := serverResCache[task.groupVersionKind()]; ok {
			serverRes = val
			err = nil
		} else {
			err = retry.OnError(retry.DefaultRetry, isRetryable, func() error {
				serverRes, err = kubeutil.ServerResourceForGroupVersionKind(sc.disco, task.groupVersionKind(), "get")
				//nolint:wrapcheck // complicated function, not wrapping to avoid failure of error type checks
				return err
			})
			if serverRes != nil {
				serverResCache[task.groupVersionKind()] = serverRes
			}
		}

		shouldSkipDryRunOnMissingResource := func() bool {
			// skip dry run on missing resource error for all application resources
			if sc.skipDryRunOnMissingResource {
				return true
			}
			return (task.targetObj != nil && resourceutil.HasAnnotationOption(task.targetObj, common.AnnotationSyncOptions, common.SyncOptionSkipDryRunOnMissingResource)) ||
				sc.hasCRDOfGroupKind(task.group(), task.kind())
		}

		if err != nil {
			switch {
			case apierrors.IsNotFound(err) && shouldSkipDryRunOnMissingResource():
				// Special case for custom resources: if CRD is not yet known by the K8s API server,
				// and the CRD is part of this sync or the resource is annotated with SkipDryRunOnMissingResource=true,
				// then skip verification during `kubectl apply --dry-run` since we expect the CRD
				// to be created during app synchronization.
				sc.log.WithValues("task", task).V(1).Info("Skip dry-run for custom resource")
				task.skipDryRun = true
			default:
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

	// for prune tasks, modify the waves for proper cleanup i.e reverse of sync wave (creation order)
	pruneTasks := make(map[int][]*syncTask)
	for _, task := range tasks {
		if task.isPrune() {
			pruneTasks[task.wave()] = append(pruneTasks[task.wave()], task)
		}
	}

	var uniquePruneWaves []int
	for k := range pruneTasks {
		uniquePruneWaves = append(uniquePruneWaves, k)
	}
	sort.Ints(uniquePruneWaves)

	// reorder waves for pruning tasks using symmetric swap on prune waves
	n := len(uniquePruneWaves)
	for i := 0; i < n/2; i++ {
		// waves to swap
		startWave := uniquePruneWaves[i]
		endWave := uniquePruneWaves[n-1-i]

		for _, task := range pruneTasks[startWave] {
			task.waveOverride = &endWave
		}

		for _, task := range pruneTasks[endWave] {
			task.waveOverride = &startWave
		}
	}

	// for pruneLast tasks, modify the wave to sync phase last wave of tasks + 1
	// to ensure proper cleanup, syncPhaseLastWave should also consider prune tasks to determine last wave
	syncPhaseLastWave := 0
	for _, task := range tasks {
		if task.phase == common.SyncPhaseSync {
			if task.wave() > syncPhaseLastWave {
				syncPhaseLastWave = task.wave()
			}
		}
	}
	syncPhaseLastWave = syncPhaseLastWave + 1

	for _, task := range tasks {
		if task.isPrune() &&
			(sc.pruneLast || resourceutil.HasAnnotationOption(task.liveObj, common.AnnotationSyncOptions, common.SyncOptionPruneLast)) {
			task.waveOverride = &syncPhaseLastWave
		}
	}

	tasks.Sort()

	// finally enrich tasks with the result
	for _, task := range tasks {
		result, ok := sc.syncRes[task.resultKey()]
		if ok {
			task.syncStatus = result.Status
			task.operationState = result.HookPhase
			task.message = result.Message
		}
	}

	return tasks, successful
}

func (sc *syncContext) autoCreateNamespace(tasks syncTasks) syncTasks {
	isNamespaceCreationNeeded := true

	var allObjs []*unstructured.Unstructured
	copy(allObjs, sc.hooks)
	for _, res := range sc.resources {
		allObjs = append(allObjs, res.Target)
	}

	for _, res := range allObjs {
		if isNamespaceWithName(res, sc.namespace) {
			isNamespaceCreationNeeded = false
			break
		}
	}

	if isNamespaceCreationNeeded {
		nsSpec := &corev1.Namespace{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: kubeutil.NamespaceKind}, ObjectMeta: metav1.ObjectMeta{Name: sc.namespace}}
		managedNs, err := kubeutil.ToUnstructured(nsSpec)
		if err == nil {
			liveObj, err := sc.kubectl.GetResource(context.TODO(), sc.config, managedNs.GroupVersionKind(), managedNs.GetName(), metav1.NamespaceNone)
			switch {
			case err == nil:
				nsTask := &syncTask{phase: common.SyncPhasePreSync, targetObj: managedNs, liveObj: liveObj}
				_, ok := sc.syncRes[nsTask.resultKey()]
				if !ok && liveObj != nil {
					sc.log.WithValues("namespace", sc.namespace).Info("Namespace already exists")
				}
				tasks = sc.appendNsTask(tasks, nsTask, managedNs, liveObj)

			case apierrors.IsNotFound(err):
				tasks = sc.appendNsTask(tasks, &syncTask{phase: common.SyncPhasePreSync, targetObj: managedNs, liveObj: nil}, managedNs, nil)
			default:
				tasks = sc.appendFailedNsTask(tasks, managedNs, fmt.Errorf("namespace auto creation failed: %w", err))
			}
		} else {
			sc.setOperationPhase(common.OperationFailed, fmt.Sprintf("namespace auto creation failed: %s", err))
		}
	}
	return tasks
}

func (sc *syncContext) appendNsTask(tasks syncTasks, preTask *syncTask, managedNs, liveNs *unstructured.Unstructured) syncTasks {
	modified, err := sc.syncNamespace(managedNs, liveNs)
	if err != nil {
		tasks = sc.appendFailedNsTask(tasks, managedNs, fmt.Errorf("namespaceModifier error: %w", err))
	} else if modified {
		tasks = append(tasks, preTask)
	}

	return tasks
}

func (sc *syncContext) appendFailedNsTask(tasks syncTasks, unstructuredObj *unstructured.Unstructured, err error) syncTasks {
	task := &syncTask{phase: common.SyncPhasePreSync, targetObj: unstructuredObj}
	sc.setResourceResult(task, common.ResultCodeSyncFailed, common.OperationError, err.Error())
	tasks = append(tasks, task)
	return tasks
}

func isNamespaceWithName(res *unstructured.Unstructured, ns string) bool {
	return isNamespaceKind(res) &&
		res.GetName() == ns
}

func isNamespaceKind(res *unstructured.Unstructured) bool {
	return res != nil &&
		res.GetObjectKind().GroupVersionKind().Group == "" &&
		res.GetKind() == kubeutil.NamespaceKind
}

func obj(a, b *unstructured.Unstructured) *unstructured.Unstructured {
	if a != nil {
		return a
	}
	return b
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
		sc.log.Info(fmt.Sprintf("Updating operation state. phase: %s -> %s, message: '%s' -> '%s'", sc.phase, phase, sc.message, message))
	}
	sc.phase = phase
	sc.message = message
}

// ensureCRDReady waits until specified CRD is ready (established condition is true).
func (sc *syncContext) ensureCRDReady(name string) error {
	err := wait.PollUntilContextTimeout(context.Background(), time.Duration(100)*time.Millisecond, crdReadinessTimeout, true, func(_ context.Context) (bool, error) {
		crd, err := sc.extensionsclientset.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			//nolint:wrapcheck // wrapped outside the retry
			return false, err
		}
		for _, condition := range crd.Status.Conditions {
			if condition.Type == apiextensionsv1.Established {
				return condition.Status == apiextensionsv1.ConditionTrue, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to ensure CRD ready: %w", err)
	}
	return nil
}

func (sc *syncContext) shouldUseServerSideApply(targetObj *unstructured.Unstructured, dryRun bool) bool {
	// if it is a dry run, disable server side apply, as the goal is to validate only the
	// yaml correctness of the rendered manifests.
	// running dry-run in server mode breaks the auto create namespace feature
	// https://github.com/argoproj/argo-cd/issues/13874
	if sc.dryRun || dryRun {
		return false
	}

	resourceHasDisableSSAAnnotation := resourceutil.HasAnnotationOption(targetObj, common.AnnotationSyncOptions, common.SyncOptionDisableServerSideApply)
	if resourceHasDisableSSAAnnotation {
		return false
	}

	return sc.serverSideApply || resourceutil.HasAnnotationOption(targetObj, common.AnnotationSyncOptions, common.SyncOptionServerSideApply)
}

// needsClientSideApplyMigration checks if a resource has fields managed by the specified manager
// that need to be migrated to the server-side apply manager
func (sc *syncContext) needsClientSideApplyMigration(liveObj *unstructured.Unstructured, fieldManager string) bool {
	if liveObj == nil || fieldManager == "" {
		return false
	}

	managedFields := liveObj.GetManagedFields()
	if len(managedFields) == 0 {
		return false
	}

	for _, field := range managedFields {
		if field.Manager == fieldManager {
			return true
		}
	}

	return false
}

// performClientSideApplyMigration performs a client-side-apply using the specified field manager.
// This moves the 'last-applied-configuration' field to be managed by the specified manager.
// The next time server-side apply is performed, kubernetes automatically migrates all fields from the manager
// that owns 'last-applied-configuration' to the manager that uses server-side apply. This will remove the
// specified manager from the resources managed fields. 'kubectl-client-side-apply' is used as the default manager.
func (sc *syncContext) performClientSideApplyMigration(targetObj *unstructured.Unstructured, fieldManager string) error {
	sc.log.WithValues("resource", kubeutil.GetResourceKey(targetObj)).V(1).Info("Performing client-side apply migration step")

	// Apply with the specified manager to set up the migration
	_, err := sc.resourceOps.ApplyResource(
		context.TODO(),
		targetObj,
		cmdutil.DryRunNone,
		false,
		false,
		false,
		fieldManager,
	)
	if err != nil {
		return fmt.Errorf("failed to perform client-side apply migration on manager %s: %w", fieldManager, err)
	}

	return nil
}

func (sc *syncContext) applyObject(t *syncTask, dryRun, validate bool) (common.ResultCode, string) {
	dryRunStrategy := cmdutil.DryRunNone
	if dryRun {
		// irrespective of the dry run mode set in the sync context, always run
		// in client dry run mode as the goal is to validate only the
		// yaml correctness of the rendered manifests.
		// running dry-run in server mode breaks the auto create namespace feature
		// https://github.com/argoproj/argo-cd/issues/13874
		dryRunStrategy = cmdutil.DryRunClient
	}

	var err error
	var message string
	shouldReplace := sc.replace || resourceutil.HasAnnotationOption(t.targetObj, common.AnnotationSyncOptions, common.SyncOptionReplace)
	force := sc.force || resourceutil.HasAnnotationOption(t.targetObj, common.AnnotationSyncOptions, common.SyncOptionForce)
	serverSideApply := sc.shouldUseServerSideApply(t.targetObj, dryRun)

	// Check if we need to perform client-side apply migration for server-side apply
	if serverSideApply && !dryRun && sc.enableClientSideApplyMigration {
		if sc.needsClientSideApplyMigration(t.liveObj, sc.clientSideApplyMigrationManager) {
			err = sc.performClientSideApplyMigration(t.targetObj, sc.clientSideApplyMigrationManager)
			if err != nil {
				return common.ResultCodeSyncFailed, fmt.Sprintf("Failed to perform client-side apply migration: %v", err)
			}
		}
	}

	if shouldReplace {
		if t.liveObj != nil {
			// Avoid using `kubectl replace` for CRDs since 'replace' might recreate resource and so delete all CRD instances.
			// The same thing applies for namespaces, which would delete the namespace as well as everything within it,
			// so we want to avoid using `kubectl replace` in that case as well.
			if kubeutil.IsCRD(t.targetObj) || t.targetObj.GetKind() == kubeutil.NamespaceKind {
				update := t.targetObj.DeepCopy()
				update.SetResourceVersion(t.liveObj.GetResourceVersion())
				_, err = sc.resourceOps.UpdateResource(context.TODO(), update, dryRunStrategy)
				if err == nil {
					message = fmt.Sprintf("%s/%s updated", t.targetObj.GetKind(), t.targetObj.GetName())
				} else {
					message = fmt.Sprintf("error when updating: %v", err.Error())
				}
			} else {
				message, err = sc.resourceOps.ReplaceResource(context.TODO(), t.targetObj, dryRunStrategy, force)
			}
		} else {
			message, err = sc.resourceOps.CreateResource(context.TODO(), t.targetObj, dryRunStrategy, validate)
		}
	} else {
		message, err = sc.resourceOps.ApplyResource(context.TODO(), t.targetObj, dryRunStrategy, force, validate, serverSideApply, sc.serverSideApplyManager)
	}
	if err != nil {
		return common.ResultCodeSyncFailed, err.Error()
	}
	if kubeutil.IsCRD(t.targetObj) && !dryRun {
		crdName := t.targetObj.GetName()
		if err = sc.ensureCRDReady(crdName); err != nil {
			sc.log.Error(err, fmt.Sprintf("failed to ensure that CRD %s is ready", crdName))
		}
	}
	return common.ResultCodeSynced, message
}

// pruneObject deletes the object if both prune is true and dryRun is false. Otherwise appropriate message
func (sc *syncContext) pruneObject(liveObj *unstructured.Unstructured, prune, dryRun bool) (common.ResultCode, string) {
	if !prune {
		return common.ResultCodePruneSkipped, "ignored (requires pruning)"
	} else if resourceutil.HasAnnotationOption(liveObj, common.AnnotationSyncOptions, common.SyncOptionDisablePrune) {
		return common.ResultCodePruneSkipped, "ignored (no prune)"
	}
	if dryRun {
		return common.ResultCodePruned, "pruned (dry run)"
	}
	// Skip deletion if object is already marked for deletion, so we don't cause a resource update hotloop
	deletionTimestamp := liveObj.GetDeletionTimestamp()
	if deletionTimestamp == nil || deletionTimestamp.IsZero() {
		err := sc.kubectl.DeleteResource(context.TODO(), sc.config, liveObj.GroupVersionKind(), liveObj.GetName(), liveObj.GetNamespace(), sc.getDeleteOptions())
		if err != nil {
			return common.ResultCodeSyncFailed, err.Error()
		}
	}
	return common.ResultCodePruned, "pruned"
}

func (sc *syncContext) getDeleteOptions() metav1.DeleteOptions {
	propagationPolicy := metav1.DeletePropagationForeground
	if sc.prunePropagationPolicy != nil {
		propagationPolicy = *sc.prunePropagationPolicy
	}
	deleteOption := metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	return deleteOption
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

func isCRDOfGroupKind(group string, kind string, obj *unstructured.Unstructured) bool {
	if kubeutil.IsCRD(obj) {
		crdGroup, ok, err := unstructured.NestedString(obj.Object, "spec", "group")
		if err != nil || !ok {
			return false
		}
		crdKind, ok, err := unstructured.NestedString(obj.Object, "spec", "names", "kind")
		if err != nil || !ok {
			return false
		}
		if group == crdGroup && crdKind == kind {
			return true
		}
	}
	return false
}

func (sc *syncContext) hasCRDOfGroupKind(group string, kind string) bool {
	for _, obj := range sc.targetObjs() {
		if isCRDOfGroupKind(group, kind, obj) {
			return true
		}
	}
	return false
}

// terminate looks for any running jobs/workflow hooks and deletes the resource
func (sc *syncContext) Terminate() {
	terminateSuccessful := true
	sc.log.V(1).Info("terminating")
	tasks, _ := sc.getSyncTasks()
	for _, task := range tasks {
		if !task.isHook() || task.liveObj == nil {
			continue
		}
		if err := sc.removeHookFinalizer(task); err != nil {
			sc.setResourceResult(task, task.syncStatus, common.OperationError, fmt.Sprintf("Failed to remove hook finalizer: %v", err))
			terminateSuccessful = false
			continue
		}
		phase, msg, err := sc.getOperationPhase(task.liveObj)
		if err != nil {
			sc.setOperationPhase(common.OperationError, fmt.Sprintf("Failed to get hook health: %v", err))
			return
		}
		if phase == common.OperationRunning {
			err := sc.deleteResource(task)
			if err != nil && !apierrors.IsNotFound(err) {
				sc.setResourceResult(task, "", common.OperationFailed, fmt.Sprintf("Failed to delete: %v", err))
				terminateSuccessful = false
			} else {
				sc.setResourceResult(task, "", common.OperationSucceeded, "Deleted")
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
	sc.log.WithValues("task", task).V(1).Info("Deleting resource")
	resIf, err := sc.getResourceIf(task, "delete")
	if err != nil {
		return err
	}
	err = resIf.Delete(context.TODO(), task.name(), sc.getDeleteOptions())
	if err != nil {
		return fmt.Errorf("failed to delete resource: %w", err)
	}
	return nil
}

func (sc *syncContext) getResourceIf(task *syncTask, verb string) (dynamic.ResourceInterface, error) {
	apiResource, err := kubeutil.ServerResourceForGroupVersionKind(sc.disco, task.groupVersionKind(), verb)
	if err != nil {
		return nil, fmt.Errorf("failed to get api resource: %w", err)
	}
	res := kubeutil.ToGroupVersionResource(task.groupVersionKind().GroupVersion().String(), apiResource)
	resIf := kubeutil.ToResourceInterface(sc.dynamicIf, apiResource, res, task.namespace())
	return resIf, nil
}

var operationPhases = map[common.ResultCode]common.OperationPhase{
	common.ResultCodeSynced:       common.OperationRunning,
	common.ResultCodeSyncFailed:   common.OperationFailed,
	common.ResultCodePruned:       common.OperationSucceeded,
	common.ResultCodePruneSkipped: common.OperationSucceeded,
}

// tri-state
type runState int

const (
	successful runState = iota
	pending
	failed
)

func (sc *syncContext) runTasks(tasks syncTasks, dryRun bool) runState {
	dryRun = dryRun || sc.dryRun

	sc.log.WithValues("numTasks", len(tasks), "dryRun", dryRun).V(1).Info("Running tasks")

	state := successful
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
		if !sc.pruneConfirmed {
			var resources []string
			for _, task := range pruneTasks {
				if resourceutil.HasAnnotationOption(task.liveObj, common.AnnotationSyncOptions, common.SyncOptionPruneRequireConfirm) {
					resources = append(resources, fmt.Sprintf("%s/%s/%s", task.obj().GetAPIVersion(), task.obj().GetKind(), task.name()))
				}
			}
			if len(resources) > 0 {
				sc.log.WithValues("resources", resources).Info("Prune requires confirmation")
				andMessage := ""
				if len(resources) > 1 {
					andMessage = fmt.Sprintf(" and %d more resources", len(resources)-1)
				}
				sc.message = fmt.Sprintf("Waiting for pruning confirmation of %s%s", resources[0], andMessage)
				return pending
			}
		}

		ss := newStateSync(state)
		for _, task := range pruneTasks {
			t := task
			ss.Go(func(state runState) runState {
				logCtx := sc.log.WithValues("dryRun", dryRun, "task", t)
				logCtx.V(1).Info("Pruning")
				result, message := sc.pruneObject(t.liveObj, sc.prune, dryRun)
				if result == common.ResultCodeSyncFailed {
					state = failed
					logCtx.WithValues("message", message).Info("Pruning failed")
				}
				if !dryRun || sc.dryRun || result == common.ResultCodeSyncFailed {
					sc.setResourceResult(t, result, operationPhases[result], message)
				}
				return state
			})
		}
		state = ss.Wait()
	}

	if state != successful {
		return state
	}

	// delete anything that need deleting
	hooksPendingDeletion := createTasks.Filter(func(t *syncTask) bool { return t.deleteBeforeCreation() })
	if hooksPendingDeletion.Len() > 0 {
		ss := newStateSync(state)
		for _, task := range hooksPendingDeletion {
			t := task
			ss.Go(func(state runState) runState {
				sc.log.WithValues("dryRun", dryRun, "task", t).V(1).Info("Deleting")
				if !dryRun {
					err := sc.deleteResource(t)
					if err != nil {
						// it is possible to get a race condition here, such that the resource does not exist when
						// delete is requested, we treat this as a nop
						if !apierrors.IsNotFound(err) {
							state = failed
							sc.setResourceResult(t, "", common.OperationError, fmt.Sprintf("failed to delete resource: %v", err))
						}
					} else {
						// if there is anything that needs deleting, we are at best now in pending and
						// want to return and wait for sync to be invoked again
						state = pending
					}
				}
				return state
			})
		}
		state = ss.Wait()
	}

	if state != successful {
		return state
	}

	// finally create resources
	var tasksGroup syncTasks
	for _, task := range createTasks {
		// Only wait if the type of the next task is different than the previous type
		if len(tasksGroup) > 0 && tasksGroup[0].targetObj.GetKind() != task.kind() {
			state = sc.processCreateTasks(state, tasksGroup, dryRun)
			tasksGroup = syncTasks{task}
		} else {
			tasksGroup = append(tasksGroup, task)
		}
	}
	if len(tasksGroup) > 0 {
		state = sc.processCreateTasks(state, tasksGroup, dryRun)
	}
	return state
}

func (sc *syncContext) processCreateTasks(state runState, tasks syncTasks, dryRun bool) runState {
	ss := newStateSync(state)
	for _, task := range tasks {
		if dryRun && task.skipDryRun {
			continue
		}
		t := task
		ss.Go(func(state runState) runState {
			logCtx := sc.log.WithValues("dryRun", dryRun, "task", t)
			logCtx.V(1).Info("Applying")
			validate := sc.validate && !resourceutil.HasAnnotationOption(t.targetObj, common.AnnotationSyncOptions, common.SyncOptionsDisableValidation)
			result, message := sc.applyObject(t, dryRun, validate)
			if result == common.ResultCodeSyncFailed {
				logCtx.WithValues("message", message).Info("Apply failed")
				state = failed
			}
			if !dryRun || sc.dryRun || result == common.ResultCodeSyncFailed {
				phase := operationPhases[result]
				// no resources are created in dry-run, so running phase means validation was
				// successful and sync operation succeeded
				if sc.dryRun && phase == common.OperationRunning {
					phase = common.OperationSucceeded
				}
				sc.setResourceResult(t, result, phase, message)
			}
			return state
		})
	}
	return ss.Wait()
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
		ResourceKey: kubeutil.GetResourceKey(task.obj()),
		Images:      kubeutil.GetResourceImages(task.obj()),
		Version:     task.version(),
		Status:      task.syncStatus,
		Message:     task.message,
		HookType:    task.hookType(),
		HookPhase:   task.operationState,
		SyncPhase:   task.phase,
	}

	logCtx := sc.log.WithValues("namespace", task.namespace(), "kind", task.kind(), "name", task.name(), "phase", task.phase)

	if ok {
		// update existing value
		if res.Status != existing.Status || res.HookPhase != existing.HookPhase || res.Message != existing.Message {
			logCtx.Info(fmt.Sprintf("Updating resource result, status: '%s' -> '%s', phase '%s' -> '%s', message '%s' -> '%s'",
				existing.Status, res.Status,
				existing.HookPhase, res.HookPhase,
				existing.Message, res.Message))
			existing.Status = res.Status
			existing.HookPhase = res.HookPhase
			existing.Message = res.Message
		}
		sc.syncRes[task.resultKey()] = existing
	} else {
		logCtx.Info(fmt.Sprintf("Adding resource result, status: '%s', phase: '%s', message: '%s'", res.Status, res.HookPhase, res.Message))
		res.Order = len(sc.syncRes) + 1
		sc.syncRes[task.resultKey()] = res
	}
}

func resourceResultKey(key kubeutil.ResourceKey, phase common.SyncPhase) string {
	return fmt.Sprintf("%s:%s", key.String(), phase)
}

type stateSync struct {
	wg           sync.WaitGroup
	results      chan runState
	currentState runState
}

func newStateSync(currentState runState) *stateSync {
	return &stateSync{
		results:      make(chan runState),
		currentState: currentState,
	}
}

func (s *stateSync) Go(f func(runState) runState) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.results <- f(s.currentState)
	}()
}

func (s *stateSync) Wait() runState {
	go func() {
		s.wg.Wait()
		close(s.results)
	}()
	res := s.currentState
	for result := range s.results {
		switch res {
		case failed:
			// Terminal state, not moving anywhere
		case pending:
			// Can only move to failed
			if result == failed {
				res = failed
			}
		case successful:
			switch result {
			case pending, failed:
				res = result
			}
		}
	}
	return res
}
