package sync

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
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

func newHook(hookType synccommon.HookType, deletePolicy synccommon.HookDeletePolicy) *unstructured.Unstructured {
	obj := testingutils.NewPod()
	obj.SetNamespace(testingutils.FakeArgoCDNamespace)
	testingutils.Annotate(obj, synccommon.AnnotationKeyHook, string(hookType))
	testingutils.Annotate(obj, synccommon.AnnotationKeyHookDeletePolicy, string(deletePolicy))
	obj.SetFinalizers([]string{hook.HookFinalizer})
	return obj
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
