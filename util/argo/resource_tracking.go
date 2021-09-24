package argo

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

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
	GetApplicationNameIfResourceBelongApp(un *unstructured.Unstructured, key string, appInformer cache.SharedIndexInformer, tm v1alpha1.TrackingMethod) string
}

type resourceTracking struct {
}

func NewResourceTracking() ResourceTracking {
	return &resourceTracking{}
}

// GetTrackingMethodFromSettings retrieve tracking method from argo-cm configmap
func GetTrackingMethodFromSettings(settingsMgr *settings.SettingsManager) v1alpha1.TrackingMethod {
	tm, err := settingsMgr.GetTrackingMethod()
	if err != nil {
		return TrackingMethodAnnotationAndLabel
	}
	return v1alpha1.TrackingMethod(tm)
}

// GetTrackingMethodFromApplicationInformer retrieve tracking method from application
func GetTrackingMethodFromApplicationInformer(appInformer cache.SharedIndexInformer, namespace, appName string) (string, error) {
	obj, exists, err := appInformer.GetIndexer().GetByKey(namespace + "/" + appName)
	app, ok := obj.(*v1alpha1.Application)
	if !exists || err != nil || !ok {
		return "", fmt.Errorf("application not found")
	}
	return string(app.Spec.TrackingMethod), nil
}

// GetTrackingMethod retrieve tracking method from application and if it is not exist take it from settings
func GetTrackingMethod(settingsMgr *settings.SettingsManager, application *v1alpha1.Application) v1alpha1.TrackingMethod {
	if application.Spec.TrackingMethod != "" {
		return application.Spec.TrackingMethod
	}
	return GetTrackingMethodFromSettings(settingsMgr)
}

// GetApplicationNameIfResourceBelongApp get app name if tracking method that defined inside application same as tm
func (rt *resourceTracking) GetApplicationNameIfResourceBelongApp(un *unstructured.Unstructured, key string, appInformer cache.SharedIndexInformer, tm v1alpha1.TrackingMethod) string {
	appName := rt.GetAppName(un, key, tm)
	if appName != "" {
		trackingMethod, err := GetTrackingMethodFromApplicationInformer(appInformer, un.GetNamespace(), appName)
		if err == nil && trackingMethod != "" && trackingMethod == string(tm) {
			return appName
		}
	}
	return ""
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

// GetAppName set label/annotation base on tracking method
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
