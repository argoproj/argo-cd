package argo

import (
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
	Get(appName, namespace string) TrackingMethod
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

func (tmu *trackingMethodUtil) Get(appName, namespace string) TrackingMethod {
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
