package argo

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/kube"
	argokube "github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	TrackingMethodAnnotation         v1alpha1.TrackingMethod = "annotation"
	TrackingMethodLabel              v1alpha1.TrackingMethod = "label"
	TrackingMethodAnnotationAndLabel v1alpha1.TrackingMethod = "annotation+label"
)

var WrongResourceTrackingFormat = fmt.Errorf("wrong resource tracking format, should be <application-name>:<group>/<kind>:<namespace>/<name>")
var LabelMaxLength = 63

// ResourceTracking defines methods which allow setup and retrieve tracking information to resource
type ResourceTracking interface {
	GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) string
	GetAppInstance(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) *AppInstanceValue
	SetAppInstance(un *unstructured.Unstructured, key, val, namespace string, trackingMethod v1alpha1.TrackingMethod) error
	BuildAppInstanceValue(value AppInstanceValue) string
	ParseAppInstanceValue(value string) (*AppInstanceValue, error)
	Normalize(config, live *unstructured.Unstructured, labelKey, trackingMethod string) error
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
	if err != nil || tm == "" {
		return TrackingMethodLabel
	}
	return v1alpha1.TrackingMethod(tm)
}

func IsOldTrackingMethod(trackingMethod string) bool {
	return trackingMethod == "" || trackingMethod == string(TrackingMethodLabel)
}

func (rt *resourceTracking) getAppInstanceValue(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) *AppInstanceValue {
	appInstanceAnnotation := argokube.GetAppInstanceAnnotation(un, common.AnnotationKeyAppInstance)
	value, err := rt.ParseAppInstanceValue(appInstanceAnnotation)
	if err != nil {
		return nil
	}
	return value
}

// GetAppName retrieve application name base on tracking method
func (rt *resourceTracking) GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) string {
	retrieveAppInstanceValue := func() string {
		value := rt.getAppInstanceValue(un, key, trackingMethod)
		if value != nil {
			return value.ApplicationName
		}
		return ""
	}
	switch trackingMethod {
	case TrackingMethodLabel:
		return argokube.GetAppInstanceLabel(un, key)
	case TrackingMethodAnnotationAndLabel:
		return retrieveAppInstanceValue()
	case TrackingMethodAnnotation:
		return retrieveAppInstanceValue()
	default:
		return argokube.GetAppInstanceLabel(un, key)
	}
}

// GetAppInstance returns the representation of the app instance annotation.
// If the tracking method does not support metadata, or the annotation could
// not be parsed, it returns nil.
func (rt *resourceTracking) GetAppInstance(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) *AppInstanceValue {
	switch trackingMethod {
	case TrackingMethodAnnotation, TrackingMethodAnnotationAndLabel:
		return rt.getAppInstanceValue(un, key, trackingMethod)
	default:
		return nil
	}
}

// UnstructuredToAppInstanceValue will build the AppInstanceValue based
// on the provided unstructured. The given namespace works as a default
// value if the resource's namespace is not defined. It should be the
// Application's target destination namespace.
func UnstructuredToAppInstanceValue(un *unstructured.Unstructured, appName, namespace string) AppInstanceValue {
	ns := un.GetNamespace()
	if ns == "" {
		ns = namespace
	}
	gvk := un.GetObjectKind().GroupVersionKind()
	return AppInstanceValue{
		ApplicationName: appName,
		Group:           gvk.Group,
		Kind:            gvk.Kind,
		Namespace:       ns,
		Name:            un.GetName(),
	}
}

// SetAppInstance set label/annotation base on tracking method
func (rt *resourceTracking) SetAppInstance(un *unstructured.Unstructured, key, val, namespace string, trackingMethod v1alpha1.TrackingMethod) error {
	setAppInstanceAnnotation := func() error {
		appInstanceValue := UnstructuredToAppInstanceValue(un, val, namespace)
		return argokube.SetAppInstanceAnnotation(un, common.AnnotationKeyAppInstance, rt.BuildAppInstanceValue(appInstanceValue))
	}
	switch trackingMethod {
	case TrackingMethodLabel:
		return argokube.SetAppInstanceLabel(un, key, val)
	case TrackingMethodAnnotation:
		return setAppInstanceAnnotation()
	case TrackingMethodAnnotationAndLabel:
		err := setAppInstanceAnnotation()
		if err != nil {
			return err
		}
		if len(val) > LabelMaxLength {
			val = val[:LabelMaxLength]
		}
		return argokube.SetAppInstanceLabel(un, key, val)
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

// Normalize updates live resource and removes diff caused but missing annotation or extra tracking label.
// The normalization is required to ensure smooth transition to new tracking method.
func (rt *resourceTracking) Normalize(config, live *unstructured.Unstructured, labelKey, trackingMethod string) error {
	if IsOldTrackingMethod(trackingMethod) {
		return nil
	}

	if live == nil || config == nil {
		return nil
	}

	label := kube.GetAppInstanceLabel(live, labelKey)
	if label == "" {
		return nil
	}

	annotation := argokube.GetAppInstanceAnnotation(config, common.AnnotationKeyAppInstance)
	err := argokube.SetAppInstanceAnnotation(live, common.AnnotationKeyAppInstance, annotation)
	if err != nil {
		return err
	}

	if argokube.GetAppInstanceLabel(config, labelKey) == "" {
		argokube.RemoveLabel(live, labelKey)
	}

	return nil
}
