package argo

import (
	"fmt"
	"strings"

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

var WrongResourceTrackingFormat = fmt.Errorf("wrong resource tracking format, should be <application-name>;<group>/<kind>/<namespace>/<name>")

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
		appInstanceAnnotation := argokube.GetAppInstanceAnnotation(un, key)
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
		return argokube.SetAppInstanceAnnotation(un, key, rt.BuildAppInstanceValue(appInstanceValue))
	default:
		return argokube.SetAppInstanceLabel(un, key, val)
	}
}

//BuildAppInstanceValue build resource tracking id in format <application-name>;<group>/<kind>/<namespace>/<name>
func (rt *resourceTracking) BuildAppInstanceValue(value AppInstanceValue) string {
	return fmt.Sprintf("%s;%s/%s/%s/%s", value.ApplicationName, value.Group, value.Kind, value.Namespace, value.Name)
}

//ParseAppInstanceValue parse resource tracking id from format <application-name>;<group>/<kind>/<namespace>/<name> to struct
func (rt *resourceTracking) ParseAppInstanceValue(value string) (*AppInstanceValue, error) {
	var appInstanceValue AppInstanceValue
	parts := strings.Split(value, ";")
	appInstanceValue.ApplicationName = parts[0]
	if len(parts) == 1 {
		return nil, WrongResourceTrackingFormat
	}
	newParts := strings.Split(parts[1], "/")
	if len(newParts) != 4 {
		return nil, WrongResourceTrackingFormat
	}
	appInstanceValue.Group = newParts[0]
	appInstanceValue.Kind = newParts[1]
	appInstanceValue.Namespace = newParts[2]
	appInstanceValue.Name = newParts[3]
	return &appInstanceValue, nil
}
