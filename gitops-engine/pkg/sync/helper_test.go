package sync

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/testing"
)

type resourceNameHealthOverride map[string]health.HealthStatusCode

func (r resourceNameHealthOverride) GetResourceHealth(obj *unstructured.Unstructured) (*health.HealthStatus, error) {
	if status, ok := r[obj.GetName()]; ok {
		return &health.HealthStatus{Status: status, Message: "test"}, nil
	}
	return nil, nil
}

func getResourceResult(resources []synccommon.ResourceSyncResult, resourceKey kube.ResourceKey) *synccommon.ResourceSyncResult {
	for _, res := range resources {
		if res.ResourceKey == resourceKey {
			return &res
		}
	}
	return nil
}

func newHook(name string, hookType synccommon.HookType, deletePolicy synccommon.HookDeletePolicy) *unstructured.Unstructured {
	obj := testingutils.NewPod()
	obj.SetName(name)
	obj.SetNamespace(testingutils.FakeArgoCDNamespace)
	testingutils.Annotate(obj, synccommon.AnnotationKeyHook, string(hookType))
	testingutils.Annotate(obj, synccommon.AnnotationKeyHookDeletePolicy, string(deletePolicy))
	obj.SetFinalizers([]string{hook.HookFinalizer})
	return obj
}

// newHelmPreSyncClusterRoleBindingHook returns a cluster-scoped PreSync hook as rendered from
// Helm pre-install annotations (testkube-style webhook cert manager).
func newHelmPreSyncClusterRoleBindingHook(name string) *unstructured.Unstructured {
	crb := testingutils.NewClusterRoleBinding()
	crb.SetName(name)
	crb.SetAnnotations(map[string]string{
		"helm.sh/hook":               "pre-install",
		"helm.sh/hook-delete-policy": "hook-succeeded",
		"helm.sh/hook-weight":        "-8",
	})
	return crb
}

// newHelmPreSyncJobHook returns a namespaced PreSync Job hook (webhook-cert-create pattern).
func newHelmPreSyncJobHook(name string) *unstructured.Unstructured {
	job := &unstructured.Unstructured{}
	job.SetGroupVersionKind(schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"})
	job.SetName(name)
	job.SetAnnotations(map[string]string{
		"helm.sh/hook":               "pre-install",
		"helm.sh/hook-delete-policy": "hook-succeeded",
		"helm.sh/hook-weight":        "-7",
	})
	return job
}

// hookResourceKey returns the ResourceKey that Argo CD will assign to this object during sync,
// including namespace enrichment for cluster-scoped resources deployed to a namespaced Application
func hookResourceKey(obj *unstructured.Unstructured, destinationNamespace string) kube.ResourceKey {
	enriched := obj.DeepCopy()
	if enriched.GetNamespace() == "" {
		enriched.SetNamespace(destinationNamespace)
	}
	return kube.GetResourceKey(enriched)
}

func withReplaceAnnotation(un *unstructured.Unstructured) *unstructured.Unstructured {
	un.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: synccommon.SyncOptionReplace})
	return un
}

func withServerSideApplyAnnotation(un *unstructured.Unstructured) *unstructured.Unstructured {
	un.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: synccommon.SyncOptionServerSideApply})
	return un
}

func withDisableServerSideApplyAnnotation(un *unstructured.Unstructured) *unstructured.Unstructured {
	un.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: synccommon.SyncOptionDisableServerSideApply})
	return un
}

func withReplaceAndServerSideApplyAnnotations(un *unstructured.Unstructured) *unstructured.Unstructured {
	un.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: "Replace=true,ServerSideApply=true"})
	return un
}

func withForceAnnotation(un *unstructured.Unstructured) *unstructured.Unstructured {
	un.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: synccommon.SyncOptionForce})
	return un
}

func withForceAndReplaceAnnotations(un *unstructured.Unstructured) *unstructured.Unstructured {
	un.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: "Force=true,Replace=true"})
	return un
}

func createNamespaceTask(namespace string) (*syncTask, error) {
	nsSpec := &corev1.Namespace{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: kube.NamespaceKind}, ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	unstructuredObj, err := kube.ToUnstructured(nsSpec)

	task := &syncTask{phase: synccommon.SyncPhasePreSync, targetObj: unstructuredObj}
	if err != nil {
		return task, fmt.Errorf("failed to convert namespace spec to unstructured: %w", err)
	}
	return task, nil
}
