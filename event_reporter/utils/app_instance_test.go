package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
)

func TestGetParentAppIdentityWithinNonControllerNs(t *testing.T) {
	resourceTracking := argo.NewResourceTracking()
	annotations := make(map[string]string)
	constrollerNs := "runtime"
	expectedName := "guestbook"
	expectedNamespace := "test-apps"

	guestbookApp := appsv1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      expectedName,
			Namespace: expectedNamespace,
		},
	}
	annotations[common.AnnotationKeyAppInstance] = resourceTracking.BuildAppInstanceValue(argo.AppInstanceValue{
		Name:            "test",
		ApplicationName: guestbookApp.InstanceName(constrollerNs),
		Group:           "group",
		Kind:            "Rollout",
		Namespace:       "test-resources",
	})
	guestbookApp.Annotations = annotations

	res := GetParentAppIdentity(&guestbookApp, common.LabelKeyAppInstance, "annotation")

	assert.Equal(t, expectedName, res.Name)
	assert.Equal(t, expectedNamespace, res.Namespace)
}

func TestGetParentAppIdentityWithinControllerNs(t *testing.T) {
	resourceTracking := argo.NewResourceTracking()
	annotations := make(map[string]string)
	constrollerNs := "runtime"
	expectedName := "guestbook"
	expectedNamespace := ""

	guestbookApp := appsv1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      expectedName,
			Namespace: constrollerNs,
		},
	}
	annotations[common.AnnotationKeyAppInstance] = resourceTracking.BuildAppInstanceValue(argo.AppInstanceValue{
		Name:            "test",
		ApplicationName: guestbookApp.InstanceName(constrollerNs),
		Group:           "group",
		Kind:            "Rollout",
		Namespace:       "test-resources",
	})
	guestbookApp.Annotations = annotations

	res := GetParentAppIdentity(&guestbookApp, common.LabelKeyAppInstance, "annotation")

	assert.Equal(t, expectedName, res.Name)
	assert.Equal(t, expectedNamespace, res.Namespace)
}
