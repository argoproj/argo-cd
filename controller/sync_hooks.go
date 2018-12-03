package controller

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/apis/batch"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/health"
	hookutil "github.com/argoproj/argo-cd/util/hook"
	"github.com/argoproj/argo-cd/util/kube"
)

// doHookSync initiates (or continues) a hook-based sync. This method will be invoked when there may
// already be in-flight (potentially incomplete) jobs/workflows, and should be idempotent.
func (sc *syncContext) doHookSync(syncTasks []syncTask, hooks []*unstructured.Unstructured) {
	if !sc.startedPreSyncPhase() {
		if !sc.verifyPermittedHooks(hooks) {
			return
		}
	}
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
		healthState, err := health.SetApplicationHealth(sc.comparison, GetLiveObjs(sc.resources))
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

// verifyPermittedHooks verifies all hooks are permitted in the project
func (sc *syncContext) verifyPermittedHooks(hooks []*unstructured.Unstructured) bool {
	for _, hook := range hooks {
		gvk := hook.GroupVersionKind()
		serverRes, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)
		if err != nil {
			sc.setOperationPhase(appv1.OperationError, fmt.Sprintf("unable to identify api resource type: %v", gvk))
			return false
		}
		if !sc.proj.IsResourcePermitted(metav1.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, serverRes.Namespaced) {
			sc.setOperationPhase(appv1.OperationFailed, fmt.Sprintf("Hook resource %s:%s is not permitted in project %s", gvk.Group, gvk.Kind, sc.proj.Name))
			return false
		}

		if serverRes.Namespaced && !sc.proj.IsDestinationPermitted(appv1.ApplicationDestination{Namespace: hook.GetNamespace(), Server: sc.server}) {
			sc.setResourceDetails(&appv1.ResourceDetails{
				Name:      hook.GetName(),
				Group:     hook.GroupVersionKind().Group,
				Kind:      hook.GetKind(),
				Namespace: hook.GetNamespace(),
				Message:   fmt.Sprintf("namespace %v is not permitted in project '%s'", hook.GetNamespace(), sc.proj.Name),
				Status:    appv1.ResourceDetailsSyncFailed,
			})
			return false
		}
	}
	return true
}

// getHooks returns all Argo CD hooks, optionally filtered by ones of the specific type(s)
func (sc *syncContext) getHooks(hookTypes ...appv1.HookType) ([]*unstructured.Unstructured, error) {
	var hooks []*unstructured.Unstructured
	for _, manifest := range sc.manifestInfo.Manifests {
		var hook unstructured.Unstructured
		err := json.Unmarshal([]byte(manifest), &hook)
		if err != nil {
			return nil, err
		}
		if hook.GetNamespace() == "" {
			hook.SetNamespace(sc.namespace)
		}
		if !hookutil.IsArgoHook(&hook) {
			// TODO: in the future, if we want to map helm hooks to Argo CD lifecycles, we should
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
				Group:     hook.GroupVersionKind().Group,
				Kind:      hook.GetKind(),
				Namespace: hook.GetNamespace(),
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

// syncNonHookTasks syncs or prunes the objects that are not handled by hooks using an apply sync.
// returns true if the sync was successful
func (sc *syncContext) syncNonHookTasks(syncTasks []syncTask) bool {
	var nonHookTasks []syncTask
	for _, task := range syncTasks {
		if task.targetObj == nil {
			nonHookTasks = append(nonHookTasks, task)
		} else {
			annotations := task.targetObj.GetAnnotations()
			if annotations != nil && annotations[common.AnnotationKeyHook] != "" {
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
	apiResource, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)
	if err != nil {
		return false, err
	}
	resource := kube.ToGroupVersionResource(gvk.GroupVersion().String(), apiResource)
	resIf := kube.ToResourceInterface(sc.dynamicIf, apiResource, resource, hook.GetNamespace())

	var liveObj *unstructured.Unstructured
	existing, err := resIf.Get(hook.GetName(), metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			return false, fmt.Errorf("Failed to get status of %s hook %s '%s': %v", hookType, gvk, hook.GetName(), err)
		}
		_, err := sc.kubectl.ApplyResource(sc.config, hook, hook.GetNamespace(), false, false)
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
		if enforceHookDeletePolicy(hook, hookStatus.Status) {
			err = sc.deleteHook(hook.GetName(), hook.GetKind(), hook.GetAPIVersion(), hook.GetNamespace())
			if err != nil {
				hookStatus.Status = appv1.OperationFailed
				hookStatus.Message = fmt.Sprintf("failed to delete %s hook: %v", hookStatus.Status, err)
			}
		}
	}
	return sc.updateHookStatus(hookStatus), nil
}

// enforceHookDeletePolicy examines the hook deletion policy of a object and deletes it based on the status
func enforceHookDeletePolicy(hook *unstructured.Unstructured, phase appv1.OperationPhase) bool {
	annotations := hook.GetAnnotations()
	if annotations == nil {
		return false
	}
	deletePolicies := strings.Split(annotations[common.AnnotationKeyHookDeletePolicy], ",")
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
	resHookTypes := strings.Split(annotations[common.AnnotationKeyHook], ",")
	for _, ht := range resHookTypes {
		if string(hookType) == strings.TrimSpace(ht) {
			return true
		}
	}
	return false
}

func newHookStatus(hook *unstructured.Unstructured, hookType appv1.HookType) appv1.HookStatus {
	hookStatus := appv1.HookStatus{
		Name:       hook.GetName(),
		Kind:       hook.GetKind(),
		APIVersion: hook.GetAPIVersion(),
		Type:       hookType,
		Namespace:  hook.GetNamespace(),
	}
	gvk := schema.FromAPIVersionAndKind(hookStatus.APIVersion, hookStatus.Kind)
	if isBatchJob(gvk) {
		updateStatusFromBatchJob(hook, &hookStatus)
	} else if isArgoWorkflow(gvk) {
		updateStatusFromArgoWorkflow(hook, &hookStatus)
	} else if isPod(gvk) {
		updateStatusFromPod(hook, &hookStatus)
	} else {
		hookStatus.Status = appv1.OperationSucceeded
		hookStatus.Message = fmt.Sprintf("%s created", hook.GetName())
	}
	return hookStatus
}

// isRunnable returns if the resource object is a runnable type which needs to be terminated
func isRunnable(hookStatus *appv1.HookStatus) bool {
	gvk := schema.FromAPIVersionAndKind(hookStatus.APIVersion, hookStatus.Kind)
	return isBatchJob(gvk) || isArgoWorkflow(gvk) || isPod(gvk)
}

func isBatchJob(gvk schema.GroupVersionKind) bool {
	return gvk.Group == "batch" && gvk.Kind == "Job"
}

func updateStatusFromBatchJob(hook *unstructured.Unstructured, hookStatus *appv1.HookStatus) {
	var job batch.Job
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(hook.Object, &job)
	if err != nil {
		hookStatus.Status = appv1.OperationError
		hookStatus.Message = err.Error()
		return
	}
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

func isArgoWorkflow(gvk schema.GroupVersionKind) bool {
	return gvk.Group == "argoproj.io" && gvk.Kind == "Workflow"
}

func updateStatusFromArgoWorkflow(hook *unstructured.Unstructured, hookStatus *appv1.HookStatus) {
	var wf wfv1.Workflow
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(hook.Object, &wf)
	if err != nil {
		hookStatus.Status = appv1.OperationError
		hookStatus.Message = err.Error()
		return
	}
	switch wf.Status.Phase {
	case wfv1.NodePending, wfv1.NodeRunning:
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

func isPod(gvk schema.GroupVersionKind) bool {
	return gvk.Group == "" && gvk.Kind == "Pod"
}

func updateStatusFromPod(hook *unstructured.Unstructured, hookStatus *appv1.HookStatus) {
	var pod apiv1.Pod
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(hook.Object, &pod)
	if err != nil {
		hookStatus.Status = appv1.OperationError
		hookStatus.Message = err.Error()
		return
	}
	getFailMessage := func(ctr *apiv1.ContainerStatus) string {
		if ctr.State.Terminated != nil {
			if ctr.State.Terminated.Message != "" {
				return ctr.State.Terminated.Message
			}
			if ctr.State.Terminated.Reason == "OOMKilled" {
				return ctr.State.Terminated.Reason
			}
			if ctr.State.Terminated.ExitCode != 0 {
				return fmt.Sprintf("container %q failed with exit code %d", ctr.Name, ctr.State.Terminated.ExitCode)
			}
		}
		return ""
	}

	switch pod.Status.Phase {
	case apiv1.PodPending, apiv1.PodRunning:
		hookStatus.Status = appv1.OperationRunning
	case apiv1.PodSucceeded:
		hookStatus.Status = appv1.OperationSucceeded
	case apiv1.PodFailed:
		hookStatus.Status = appv1.OperationFailed
		if pod.Status.Message != "" {
			// Pod has a nice error message. Use that.
			hookStatus.Message = pod.Status.Message
			return
		}
		for _, ctr := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
			if msg := getFailMessage(&ctr); msg != "" {
				hookStatus.Message = msg
				return
			}
		}
	case apiv1.PodUnknown:
		hookStatus.Status = appv1.OperationError
	}
}

func (sc *syncContext) getHookStatus(hookObj *unstructured.Unstructured, hookType appv1.HookType) *appv1.HookStatus {
	for _, hr := range sc.syncRes.Hooks {
		if hr.Name == hookObj.GetName() && hr.Kind == hookObj.GetKind() && hr.Type == hookType {
			return hr
		}
	}
	return nil
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
		if isRunnable(hookStatus) {
			hookStatus.Status = appv1.OperationFailed
			err := sc.deleteHook(hookStatus.Name, hookStatus.Kind, hookStatus.APIVersion, hookStatus.Namespace)
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

func (sc *syncContext) deleteHook(name, kind, apiVersion string, namespace string) error {
	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	apiResource, err := kube.ServerResourceForGroupVersionKind(sc.disco, gvk)
	if err != nil {
		return err
	}
	resource := kube.ToGroupVersionResource(gvk.GroupVersion().String(), apiResource)
	resIf := kube.ToResourceInterface(sc.dynamicIf, apiResource, resource, namespace)
	propagationPolicy := metav1.DeletePropagationForeground
	return resIf.Delete(name, &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}
