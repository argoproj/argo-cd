package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/v3/util/lua"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type HookType string

const (
	PreDeleteHookType  HookType = "PreDelete"
	PostDeleteHookType HookType = "PostDelete"
)

var hookTypeAnnotations = map[HookType]map[string]string{
	PreDeleteHookType: {
		"argocd.argoproj.io/hook": string(PreDeleteHookType),
		"helm.sh/hook":            "pre-delete",
	},
	PostDeleteHookType: {
		"argocd.argoproj.io/hook": string(PostDeleteHookType),
		"helm.sh/hook":            "post-delete",
	},
}

func isHookOfType(obj *unstructured.Unstructured, hookType HookType) bool {
	if obj == nil || obj.GetAnnotations() == nil {
		return false
	}

	for k, v := range hookTypeAnnotations[hookType] {
		if val, ok := obj.GetAnnotations()[k]; ok && val == v {
			return true
		}
	}
	return false
}

func isHook(obj *unstructured.Unstructured) bool {
	if hook.IsHook(obj) {
		return true
	}

	for hookType := range hookTypeAnnotations {
		if isHookOfType(obj, hookType) {
			return true
		}
	}
	return false
}

func isPreDeleteHook(obj *unstructured.Unstructured) bool {
	return isHookOfType(obj, PreDeleteHookType)
}

func isPostDeleteHook(obj *unstructured.Unstructured) bool {
	return isHookOfType(obj, PostDeleteHookType)
}

// executeHooks is a generic function to execute hooks of a specified type
func (ctrl *ApplicationController) executeHooks(hookType HookType, app *appv1.Application, proj *appv1.AppProject, liveObjs map[kube.ResourceKey]*unstructured.Unstructured, config *rest.Config, logCtx *log.Entry) (bool, error) {
	appLabelKey, err := ctrl.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return false, err
	}

	var revisions []string
	for _, src := range app.Spec.GetSources() {
		revisions = append(revisions, src.TargetRevision)
	}

	targets, _, _, err := ctrl.appStateManager.GetRepoObjs(context.Background(), app, app.Spec.GetSources(), appLabelKey, revisions, false, false, false, proj, true)
	if err != nil {
		return false, err
	}

	// Find existing hooks of the specified type
	runningHooks := map[kube.ResourceKey]*unstructured.Unstructured{}
	for key, obj := range liveObjs {
		if isHookOfType(obj, hookType) {
			runningHooks[key] = obj
		}
	}

	// Find expected hooks that need to be created
	expectedHook := map[kube.ResourceKey]*unstructured.Unstructured{}
	for _, obj := range targets {
		if obj.GetNamespace() == "" {
			obj.SetNamespace(app.Spec.Destination.Namespace)
		}
		if !isHookOfType(obj, hookType) {
			continue
		}
		if runningHook := runningHooks[kube.GetResourceKey(obj)]; runningHook == nil {
			expectedHook[kube.GetResourceKey(obj)] = obj
		}
	}

	// Create hooks that don't exist yet
	createdCnt := 0
	for _, obj := range expectedHook {
		// Add app instance label so the hook can be tracked and cleaned up
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[appLabelKey] = app.InstanceName(ctrl.namespace)
		obj.SetLabels(labels)

		_, err = ctrl.kubectl.CreateResource(context.Background(), config, obj.GroupVersionKind(), obj.GetName(), obj.GetNamespace(), obj, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}
		createdCnt++
	}

	if createdCnt > 0 {
		logCtx.Infof("Created %d %s hooks", createdCnt, hookType)
		return false, nil
	}

	// Check health of running hooks
	resourceOverrides, err := ctrl.settingsMgr.GetResourceOverrides()
	if err != nil {
		return false, err
	}
	healthOverrides := lua.ResourceHealthOverrides(resourceOverrides)

	progressingHooksCount := 0
	var failedHooks []string
	var failedHookObjects []*unstructured.Unstructured
	for _, obj := range runningHooks {
		hookHealth, err := health.GetResourceHealth(obj, healthOverrides)
		if err != nil {
			return false, err
		}
		if hookHealth == nil {
			logCtx.WithFields(log.Fields{
				"group":     obj.GroupVersionKind().Group,
				"version":   obj.GroupVersionKind().Version,
				"kind":      obj.GetKind(),
				"name":      obj.GetName(),
				"namespace": obj.GetNamespace(),
			}).Info("No health check defined for resource, considering it healthy")
			hookHealth = &health.HealthStatus{
				Status: health.HealthStatusHealthy,
			}
		}
		switch hookHealth.Status {
		case health.HealthStatusProgressing:
			progressingHooksCount++
		case health.HealthStatusDegraded:
			failedHooks = append(failedHooks, fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName()))
			failedHookObjects = append(failedHookObjects, obj)
		}
	}

	if len(failedHooks) > 0 {
		// Delete failed hooks to allow retry with potentially fixed hook definitions
		logCtx.Infof("Deleting %d failed %s hook(s) to allow retry", len(failedHookObjects), hookType)
		for _, obj := range failedHookObjects {
			err = ctrl.kubectl.DeleteResource(context.Background(), config, obj.GroupVersionKind(), obj.GetName(), obj.GetNamespace(), metav1.DeleteOptions{})
			if err != nil {
				logCtx.WithError(err).Warnf("Failed to delete failed hook %s/%s", obj.GetNamespace(), obj.GetName())
			}
		}
		return false, fmt.Errorf("%s hook(s) failed: %s", hookType, strings.Join(failedHooks, ", "))
	}

	if progressingHooksCount > 0 {
		logCtx.Infof("Waiting for %d %s hooks to complete", progressingHooksCount, hookType)
		return false, nil
	}

	return true, nil
}

// cleanupHooks is a generic function to clean up hooks of a specified type
func (ctrl *ApplicationController) cleanupHooks(hookType HookType, liveObjs map[kube.ResourceKey]*unstructured.Unstructured, config *rest.Config, logCtx *log.Entry) (bool, error) {
	resourceOverrides, err := ctrl.settingsMgr.GetResourceOverrides()
	if err != nil {
		return false, err
	}
	healthOverrides := lua.ResourceHealthOverrides(resourceOverrides)

	pendingDeletionCount := 0
	aggregatedHealth := health.HealthStatusHealthy
	var hooks []*unstructured.Unstructured

	// Collect hooks and determine overall health
	for _, obj := range liveObjs {
		if !isHookOfType(obj, hookType) {
			continue
		}
		hookHealth, err := health.GetResourceHealth(obj, healthOverrides)
		if err != nil {
			return false, err
		}
		if hookHealth == nil {
			hookHealth = &health.HealthStatus{
				Status: health.HealthStatusHealthy,
			}
		}
		if health.IsWorse(aggregatedHealth, hookHealth.Status) {
			aggregatedHealth = hookHealth.Status
		}
		hooks = append(hooks, obj)
	}

	// Process hooks for deletion
	for _, obj := range hooks {
		deletePolicies := hook.DeletePolicies(obj)
		shouldDelete := false

		if len(deletePolicies) == 0 {
			// If no delete policy is specified, always delete hooks during cleanup phase
			shouldDelete = true
		} else {
			// Check if any delete policy matches the current hook state
			for _, policy := range deletePolicies {
				if (policy == common.HookDeletePolicyHookFailed && aggregatedHealth == health.HealthStatusDegraded) ||
					(policy == common.HookDeletePolicyHookSucceeded && aggregatedHealth == health.HealthStatusHealthy) {
					shouldDelete = true
					break
				}
			}
		}

		if shouldDelete {
			pendingDeletionCount++
			if obj.GetDeletionTimestamp() != nil {
				continue
			}
			logCtx.Infof("Deleting %s hook %s/%s", hookType, obj.GetNamespace(), obj.GetName())
			err = ctrl.kubectl.DeleteResource(context.Background(), config, obj.GroupVersionKind(), obj.GetName(), obj.GetNamespace(), metav1.DeleteOptions{})
			if err != nil {
				return false, err
			}
		}
	}

	if pendingDeletionCount > 0 {
		logCtx.Infof("Waiting for %d %s hooks to be deleted", pendingDeletionCount, hookType)
		return false, nil
	}

	return true, nil
}

// Execute and cleanup hooks for pre-delete and post-delete operations

func (ctrl *ApplicationController) executePreDeleteHooks(app *appv1.Application, proj *appv1.AppProject, liveObjs map[kube.ResourceKey]*unstructured.Unstructured, config *rest.Config, logCtx *log.Entry) (bool, error) {
	return ctrl.executeHooks(PreDeleteHookType, app, proj, liveObjs, config, logCtx)
}

func (ctrl *ApplicationController) cleanupPreDeleteHooks(liveObjs map[kube.ResourceKey]*unstructured.Unstructured, config *rest.Config, logCtx *log.Entry) (bool, error) {
	return ctrl.cleanupHooks(PreDeleteHookType, liveObjs, config, logCtx)
}

func (ctrl *ApplicationController) executePostDeleteHooks(app *appv1.Application, proj *appv1.AppProject, liveObjs map[kube.ResourceKey]*unstructured.Unstructured, config *rest.Config, logCtx *log.Entry) (bool, error) {
	return ctrl.executeHooks(PostDeleteHookType, app, proj, liveObjs, config, logCtx)
}

func (ctrl *ApplicationController) cleanupPostDeleteHooks(liveObjs map[kube.ResourceKey]*unstructured.Unstructured, config *rest.Config, logCtx *log.Entry) (bool, error) {
	return ctrl.cleanupHooks(PostDeleteHookType, liveObjs, config, logCtx)
}
