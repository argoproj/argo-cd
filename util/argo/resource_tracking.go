package argo

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v2/common"

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

var WrongResourceTrackingFormat = fmt.Errorf("wrong resource tracking format, should be <application-name>:<group>/<kind>:<namespace>/<name>")

// ResourceTracking defines methods which allow setup and retrieve tracking information to resource
type ResourceTracking interface {
	GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) string
	SetAppInstance(un *unstructured.Unstructured, key, val, namespace string, trackingMethod v1alpha1.TrackingMethod) error
	BuildAppInstanceValue(value AppInstanceValue) string
	ParseAppInstanceValue(value string) (*AppInstanceValue, error)
}

//AppInstanceValue store information about resource tracking info
type AppInstanceValue struct {
	ApplicationName string
	Group           string
	Kind            string
	Namespace       string
	Name            string
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
		appInstanceAnnotation := argokube.GetAppInstanceAnnotation(un, common.AnnotationKeyAppInstance)
		value, err := rt.ParseAppInstanceValue(appInstanceAnnotation)
		if err != nil {
			return ""
		}
		return value.ApplicationName
	default:
		return argokube.GetAppInstanceLabel(un, key)
	}
}

// SetAppInstance set label/annotation base on tracking method
func (rt *resourceTracking) SetAppInstance(un *unstructured.Unstructured, key, val, namespace string, trackingMethod v1alpha1.TrackingMethod) error {
	switch trackingMethod {
	case TrackingMethodLabel:
		return argokube.SetAppInstanceLabel(un, key, val)
	case TrackingMethodAnnotation:
		ns := un.GetNamespace()
		if ns == "" {
			ns = namespace
		}
		gvk := un.GetObjectKind().GroupVersionKind()
		appInstanceValue := AppInstanceValue{
			ApplicationName: val,
			Group:           gvk.Group,
			Kind:            gvk.Kind,
			Namespace:       ns,
			Name:            un.GetName(),
		}
		return argokube.SetAppInstanceAnnotation(un, common.AnnotationKeyAppInstance, rt.BuildAppInstanceValue(appInstanceValue))
	default:
		return argokube.SetAppInstanceLabel(un, key, val)
	}
}

//BuildAppInstanceValue build resource tracking id in format <application-name>;<group>/<kind>/<namespace>/<name>
func (rt *resourceTracking) BuildAppInstanceValue(value AppInstanceValue) string {
	return fmt.Sprintf("%s:%s/%s:%s/%s", value.ApplicationName, value.Group, value.Kind, value.Namespace, value.Name)
}

//ParseAppInstanceValue parse resource tracking id from format <application-name>:<group>/<kind>:<namespace>/<name> to struct
func (rt *resourceTracking) ParseAppInstanceValue(value string) (*AppInstanceValue, error) {
	var appInstanceValue AppInstanceValue
	parts := strings.Split(value, ":")
	appInstanceValue.ApplicationName = parts[0]
	if len(parts) != 3 {
		return nil, WrongResourceTrackingFormat
	}
	groupParts := strings.Split(parts[1], "/")
	if len(groupParts) != 2 {
		return nil, WrongResourceTrackingFormat
	}
	nsParts := strings.Split(parts[2], "/")
	if len(nsParts) != 2 {
		return nil, WrongResourceTrackingFormat
	}
	appInstanceValue.Group = groupParts[0]
	appInstanceValue.Kind = groupParts[1]
	appInstanceValue.Namespace = nsParts[0]
	appInstanceValue.Name = nsParts[1]
	return &appInstanceValue, nil
}
