package argo

import (
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

// ResourceTracking defines methods which allow setup and retrieve tracking information to resource
type ResourceTracking interface {
	GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) string
	SetAppInstance(un *unstructured.Unstructured, key, val string, trackingMethod v1alpha1.TrackingMethod) error
}

type resourceTracking struct {
}

func NewResourceTracking() ResourceTracking {
	return &resourceTracking{}
}

// GetTrackingMethod retrieve tracking method from settings
func GetTrackingMethod(settingsMgr *settings.SettingsManager) v1alpha1.TrackingMethod {
	tm, err := settingsMgr.GetTrackingMethod()
	if err != nil {
		return TrackingMethodAnnotationAndLabel
	}
	return v1alpha1.TrackingMethod(tm)
}

// GetAppName retrieve application name base on tracking method
func (rt *resourceTracking) GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) string {
	switch trackingMethod {
	case TrackingMethodLabel:
		return argokube.GetAppInstanceLabel(un, key)
	case TrackingMethodAnnotation:
		return argokube.GetAppInstanceAnnotation(un, key)
	default:
		return argokube.GetAppInstanceLabel(un, key)
	}
}

// SetAppInstance set label/annotation base on tracking method
func (rt *resourceTracking) SetAppInstance(un *unstructured.Unstructured, key, val string, trackingMethod v1alpha1.TrackingMethod) error {
	switch trackingMethod {
	case TrackingMethodLabel:
		return argokube.SetAppInstanceLabel(un, key, val)
	case TrackingMethodAnnotation:
		return argokube.SetAppInstanceAnnotation(un, key, val)
	default:
		return argokube.SetAppInstanceLabel(un, key, val)
	}
}
