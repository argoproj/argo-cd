package argo

import (
	"encoding/json"
	"log"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/util/settings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	argokube "github.com/argoproj/argo-cd/v2/util/kube"
)

const (
	TrackingMethodAnnotation         v1alpha1.TrackingMethod = "annotation"
	TrackingMethodLabel              v1alpha1.TrackingMethod = "label"
	TrackingMethodAnnotationAndLabel v1alpha1.TrackingMethod = "annotation+label"
)

type ResourceTracking interface {
	GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) string
	SetAppInstance(un *unstructured.Unstructured, key, val string, trackingMethod v1alpha1.TrackingMethod) error
}

type resourceTracking struct {
}

func NewResourceTracking() ResourceTracking {
	return &resourceTracking{}
}

func GetTrackingMethod(settingsMgr *settings.SettingsManager) v1alpha1.TrackingMethod {
	tm, err := settingsMgr.GetTrackingMethod()
	if err != nil {
		return TrackingMethodAnnotationAndLabel
	}

	return ToTrackingMethod(tm)
}

// TODO: Remove after https://github.com/argoproj/gitops-engine/pull/330 get merged
func GetAppInstanceAnnotation(un *unstructured.Unstructured, key string) string {
	if annotations := un.GetAnnotations(); annotations != nil {
		return annotations[key]
	}
	return ""
}

func (rt *resourceTracking) GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) string {
	log.Println("Get app name, tracking method: " + trackingMethod)
	switch trackingMethod {
	case TrackingMethodLabel:
		return kube.GetAppInstanceLabel(un, key)
	case TrackingMethodAnnotation:
		log.Println("Choose annotation strategy")
		un2, _ := json.Marshal(un)

		log.Println("resource: " + string(un2))

		return GetAppInstanceAnnotation(un, key)
	default:
		return kube.GetAppInstanceLabel(un, key)
	}
}

func (rt *resourceTracking) SetAppInstance(un *unstructured.Unstructured, key, val string, trackingMethod v1alpha1.TrackingMethod) error {
	log.Println("Set app instance, tracking method: " + trackingMethod)
	switch trackingMethod {
	case TrackingMethodLabel:
		return argokube.SetAppInstanceLabel(un, key, val)
	case TrackingMethodAnnotation:
		return argokube.SetAppInstanceAnnotation(un, key, val)
	default:
		return argokube.SetAppInstanceLabel(un, key, val)
	}
}

func ToTrackingMethod(trackingMethod string) v1alpha1.TrackingMethod {
	switch os := trackingMethod; os {
	case "label":
		return TrackingMethodLabel
	case "annotation":
		return TrackingMethodAnnotation
	default:
		return TrackingMethodAnnotationAndLabel
	}
}
