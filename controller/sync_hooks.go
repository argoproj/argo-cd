package controller

import (
	"fmt"
	"github.com/argoproj/argo-cd/util/hook"
	"strings"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/apis/batch"

	"github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func isHook(obj *unstructured.Unstructured) bool {
	return hook.IsArgoHook(obj)
}

// enforceHookDeletePolicy examines the hook deletion policy of a object and deletes it based on the status
func enforceHookDeletePolicy(hook *unstructured.Unstructured, operation OperationPhase) bool {

	annotations := hook.GetAnnotations()
	if annotations == nil {
		return false
	}
	deletePolicies := strings.Split(annotations[common.AnnotationKeyHookDeletePolicy], ",")
	for _, dp := range deletePolicies {
		policy := HookDeletePolicy(strings.TrimSpace(dp))
		if policy == HookDeletePolicyHookSucceeded && operation == OperationSucceeded {
			return true
		}
		if policy == HookDeletePolicyHookFailed && operation == OperationFailed {
			return true
		}
	}
	return false
}

// getOperationPhase returns a hook status from an _live_ unstructured object
func getOperationPhase(hook *unstructured.Unstructured) (operation OperationPhase, message string) {
	gvk := hook.GroupVersionKind()
	if isBatchJob(gvk) {
		return getStatusFromBatchJob(hook)
	} else if isArgoWorkflow(gvk) {
		return getStatusFromArgoWorkflow(hook)
	} else if isPod(gvk) {
		return getStatusFromPod(hook)
	} else {
		return OperationSucceeded, fmt.Sprintf("%s created", hook.GetName())
	}
}

// isRunnable returns if the resource object is a runnable type which needs to be terminated
func isRunnable(gvk schema.GroupVersionKind) bool {
	return isBatchJob(gvk) || isArgoWorkflow(gvk) || isPod(gvk)
}

func isBatchJob(gvk schema.GroupVersionKind) bool {
	return gvk.Group == "batch" && gvk.Kind == "Job"
}

// TODO this is a copy-and-paste of health.getJobHealth(), refactor out?
func getStatusFromBatchJob(hook *unstructured.Unstructured) (operation OperationPhase, message string) {
	var job batch.Job
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(hook.Object, &job)
	if err != nil {
		return OperationError, err.Error()
	}
	failed := false
	var failMsg string
	complete := false
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
		return OperationRunning, message
	} else if failed {
		return OperationFailed, failMsg
	} else {
		return OperationSucceeded, message
	}
}

func isArgoWorkflow(gvk schema.GroupVersionKind) bool {
	return gvk.Group == "argoproj.io" && gvk.Kind == "Workflow"
}

// TODO - should we move this to health.go?
func getStatusFromArgoWorkflow(hook *unstructured.Unstructured) (operation OperationPhase, message string) {
	var wf wfv1.Workflow
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(hook.Object, &wf)
	if err != nil {
		return OperationError, err.Error()
	}
	switch wf.Status.Phase {
	case wfv1.NodePending, wfv1.NodeRunning:
		return OperationRunning, wf.Status.Message
	case wfv1.NodeSucceeded:
		return OperationSucceeded, wf.Status.Message
	case wfv1.NodeFailed:
		return OperationFailed, wf.Status.Message
	case wfv1.NodeError:
		return OperationError, wf.Status.Message
	}
	return OperationSucceeded, wf.Status.Message
}

func isPod(gvk schema.GroupVersionKind) bool {
	return gvk.Group == "" && gvk.Kind == "Pod"
}

// TODO - this is very similar to health.getPodHealth() should we use that instead?
func getStatusFromPod(hook *unstructured.Unstructured) (operation OperationPhase, message string) {
	var pod apiv1.Pod
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(hook.Object, &pod)
	if err != nil {
		return OperationError, err.Error()
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
		return OperationRunning, ""
	case apiv1.PodSucceeded:
		return OperationSucceeded, ""
	case apiv1.PodFailed:
		if pod.Status.Message != "" {
			// Pod has a nice error message. Use that.
			message = pod.Status.Message
			return
		}
		for _, ctr := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
			if msg := getFailMessage(&ctr); msg != "" {
				message = msg
				return
			}
		}
		return OperationFailed, message
	case apiv1.PodUnknown:
		return OperationError, ""
	}
	return OperationRunning, ""
}
