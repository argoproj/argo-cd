package argo

import (
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v2/util/settings"
)

type TrackingMethod string

const (
	TrackingMethodAnnotation         TrackingMethod = "annotation"
	TrackingMethodLabel              TrackingMethod = "label"
	TrackingMethodAnnotationAndLabel TrackingMethod = "annotation+label"
)

type TrackingMethodUtil interface {
	GetAppName(un *unstructured.Unstructured, key string) string
}

type trackingMethodUtil struct {
	appInformer cache.SharedIndexInformer
	settingsMgr *settings.SettingsManager
}

func NewTrackingMethodUtil(
	appInformer cache.SharedIndexInformer,
	settingsMgr *settings.SettingsManager) TrackingMethodUtil {
	return &trackingMethodUtil{
		appInformer: appInformer,
		settingsMgr: settingsMgr,
	}
}

func (tmu *trackingMethodUtil) get(appName, namespace string) TrackingMethod {
	//obj, exists, err := tmu.appInformer.GetIndexer().GetByKey(namespace + "/" + appName)
	//app, ok := obj.(*appv1.Application)
	//if !exists && err != nil && !ok {
	//	return TrackingMethodAnnotationAndLabel
	//}
	//
	//if app.Spec.TrackingMethod != "" {
	//	return app.Spec.TrackingMethod
	//}

	tm, err := tmu.settingsMgr.GetTrackingMethod()
	if err != nil {
		return TrackingMethodAnnotationAndLabel
	}

	switch os := tm; os {
	case "label":
		return TrackingMethodLabel
	case "annotation":
		return TrackingMethodAnnotation
	default:
		return TrackingMethodAnnotationAndLabel
	}
}

// TODO: Remove after https://github.com/argoproj/gitops-engine/pull/330 get merged
func GetAppInstanceAnnotation(un *unstructured.Unstructured, key string) string {
	if annotations := un.GetAnnotations(); annotations != nil {
		return annotations[key]
	}
	return ""
}

func (c *trackingMethodUtil) GetAppName(un *unstructured.Unstructured, key string) string {
	switch c.get("", "") {
	case TrackingMethodLabel:
		return kube.GetAppInstanceLabel(un, key)
	case TrackingMethodAnnotation:
		return GetAppInstanceAnnotation(un, key)
	default:
		return kube.GetAppInstanceLabel(un, key)
	}
}
