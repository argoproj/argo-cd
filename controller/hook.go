package controller

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/v2/util/lua"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var (
	postDeleteHook  = "PostDelete"
	postDeleteHooks = map[string]string{
		"argocd.argoproj.io/hook": postDeleteHook,
		"helm.sh/hook":            "post-delete",
	}
)

func isHook(obj *unstructured.Unstructured) bool {
	return hook.IsHook(obj) || isPostDeleteHook(obj)
}

func isPostDeleteHook(obj *unstructured.Unstructured) bool {
	if obj == nil || obj.GetAnnotations() == nil {
		return false
	}
	for k, v := range postDeleteHooks {
		if val, ok := obj.GetAnnotations()[k]; ok && val == v {
			return true
		}
	}
	return false
}

func (ctrl *ApplicationController) executePostDeleteHooks(app *v1alpha1.Application, proj *v1alpha1.AppProject, liveObjs map[kube.ResourceKey]*unstructured.Unstructured, config *rest.Config, logCtx *log.Entry) (bool, error) {
	appLabelKey, err := ctrl.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return false, err
	}
	var revisions []string
	for _, src := range app.Spec.GetSources() {
		revisions = append(revisions, src.TargetRevision)
	}

	targets, _, _, err := ctrl.appStateManager.GetRepoObjs(app, app.Spec.GetSources(), appLabelKey, revisions, false, false, false, proj, false)
	if err != nil {
		return false, err
	}
	runningHooks := map[kube.ResourceKey]*unstructured.Unstructured{}
	for key, obj := range liveObjs {
		if isPostDeleteHook(obj) {
			runningHooks[key] = obj
		}
	}

	expectedHook := map[kube.ResourceKey]*unstructured.Unstructured{}
	for _, obj := range targets {
		if obj.GetNamespace() == "" {
			obj.SetNamespace(app.Spec.Destination.Namespace)
		}
		if !isPostDeleteHook(obj) {
			continue
		}
		if runningHook := runningHooks[kube.GetResourceKey(obj)]; runningHook == nil {
			expectedHook[kube.GetResourceKey(obj)] = obj
		}
	}
	createdCnt := 0
	for _, obj := range expectedHook {
		_, err = ctrl.kubectl.CreateResource(context.Background(), config, obj.GroupVersionKind(), obj.GetName(), obj.GetNamespace(), obj, v1.CreateOptions{})
		if err != nil {
			return false, err
		}
		createdCnt++
	}
	if createdCnt > 0 {
		logCtx.Infof("Created %d post-delete hooks", createdCnt)
		return false, nil
	}
	resourceOverrides, err := ctrl.settingsMgr.GetResourceOverrides()
	if err != nil {
		return false, err
	}
	healthOverrides := lua.ResourceHealthOverrides(resourceOverrides)

	progressingHooksCnt := 0
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
		if hookHealth.Status == health.HealthStatusProgressing {
			progressingHooksCnt++
		}
	}
	if progressingHooksCnt > 0 {
		logCtx.Infof("Waiting for %d post-delete hooks to complete", progressingHooksCnt)
		return false, nil
	}

	return true, nil
}

func (ctrl *ApplicationController) cleanupPostDeleteHooks(liveObjs map[kube.ResourceKey]*unstructured.Unstructured, config *rest.Config, logCtx *log.Entry) (bool, error) {
	resourceOverrides, err := ctrl.settingsMgr.GetResourceOverrides()
	if err != nil {
		return false, err
	}
	healthOverrides := lua.ResourceHealthOverrides(resourceOverrides)

	pendingDeletionCount := 0
	aggregatedHealth := health.HealthStatusHealthy
	var hooks []*unstructured.Unstructured
	for _, obj := range liveObjs {
		if !isPostDeleteHook(obj) {
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

	for _, obj := range hooks {
		for _, policy := range hook.DeletePolicies(obj) {
			if policy == common.HookDeletePolicyHookFailed && aggregatedHealth == health.HealthStatusDegraded || policy == common.HookDeletePolicyHookSucceeded && aggregatedHealth == health.HealthStatusHealthy {
				pendingDeletionCount++
				if obj.GetDeletionTimestamp() != nil {
					continue
				}
				logCtx.Infof("Deleting post-delete hook %s/%s", obj.GetNamespace(), obj.GetName())
				err = ctrl.kubectl.DeleteResource(context.Background(), config, obj.GroupVersionKind(), obj.GetName(), obj.GetNamespace(), v1.DeleteOptions{})
				if err != nil {
					return false, err
				}
			}
		}
	}
	if pendingDeletionCount > 0 {
		logCtx.Infof("Waiting for %d post-delete hooks to be deleted", pendingDeletionCount)
		return false, nil
	}
	return true, nil
}
