package argo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/kube"
)

var (
	ErrWrongResourceTrackingFormat = errors.New("wrong resource tracking format, should be <application-name>:<group>/<kind>:<namespace>/<name>")
	LabelMaxLength                 = 63
	OkEndPattern                   = regexp.MustCompile("[a-zA-Z0-9]$")
)

// ResourceTracking defines methods which allow setup and retrieve tracking information to resource
type ResourceTracking interface {
	GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod, installationID string) string
	GetAppInstance(un *unstructured.Unstructured, trackingMethod v1alpha1.TrackingMethod, installationID string) *AppInstanceValue
	SetAppInstance(un *unstructured.Unstructured, key, val, namespace string, trackingMethod v1alpha1.TrackingMethod, instanceID string) error
	BuildAppInstanceValue(value AppInstanceValue) string
	ParseAppInstanceValue(value string) (*AppInstanceValue, error)
	Normalize(config, live *unstructured.Unstructured, labelKey, trackingMethod string) error
}

// AppInstanceValue store information about resource tracking info
type AppInstanceValue struct {
	ApplicationName string
	Group           string
	Kind            string
	Namespace       string
	Name            string
}

type resourceTracking struct{}

func NewResourceTracking() ResourceTracking {
	return &resourceTracking{}
}

func IsOldTrackingMethod(trackingMethod string) bool {
	return trackingMethod == "" || trackingMethod == string(v1alpha1.TrackingMethodLabel)
}

func (rt *resourceTracking) getAppInstanceValue(un *unstructured.Unstructured, installationID string) *AppInstanceValue {
	if installationID != "" && un.GetAnnotations() == nil || un.GetAnnotations()[common.AnnotationInstallationID] != installationID {
		return nil
	}
	appInstanceAnnotation, err := kube.GetAppInstanceAnnotation(un, common.AnnotationKeyAppInstance)
	if err != nil {
		return nil
	}
	value, err := rt.ParseAppInstanceValue(appInstanceAnnotation)
	if err != nil {
		return nil
	}
	return value
}

// GetAppName retrieve application name base on tracking method
func (rt *resourceTracking) GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod, instanceID string) string {
	retrieveAppInstanceValue := func() string {
		value := rt.getAppInstanceValue(un, instanceID)
		if value != nil {
			return value.ApplicationName
		}
		return ""
	}
	switch trackingMethod {
	case v1alpha1.TrackingMethodLabel:
		label, err := kube.GetAppInstanceLabel(un, key)
		if err != nil {
			return ""
		}
		return label
	case v1alpha1.TrackingMethodAnnotationAndLabel:
		return retrieveAppInstanceValue()
	case v1alpha1.TrackingMethodAnnotation:
		return retrieveAppInstanceValue()
	default:
		return retrieveAppInstanceValue()
	}
}

// GetAppInstance returns the representation of the app instance annotation.
// If the tracking method does not support metadata, or the annotation could
// not be parsed, it returns nil.
func (rt *resourceTracking) GetAppInstance(un *unstructured.Unstructured, trackingMethod v1alpha1.TrackingMethod, instanceID string) *AppInstanceValue {
	switch trackingMethod {
	case v1alpha1.TrackingMethodAnnotation, v1alpha1.TrackingMethodAnnotationAndLabel:
		return rt.getAppInstanceValue(un, instanceID)
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
func (rt *resourceTracking) SetAppInstance(un *unstructured.Unstructured, key, val, namespace string, trackingMethod v1alpha1.TrackingMethod, instanceID string) error {
	setAppInstanceAnnotation := func() error {
		appInstanceValue := UnstructuredToAppInstanceValue(un, val, namespace)
		if instanceID != "" {
			if err := kube.SetAppInstanceAnnotation(un, common.AnnotationInstallationID, instanceID); err != nil {
				return err
			}
		} else {
			if err := kube.RemoveAnnotation(un, common.AnnotationInstallationID); err != nil {
				return err
			}
		}
		return kube.SetAppInstanceAnnotation(un, common.AnnotationKeyAppInstance, rt.BuildAppInstanceValue(appInstanceValue))
	}
	switch trackingMethod {
	case v1alpha1.TrackingMethodLabel:
		err := kube.SetAppInstanceLabel(un, key, val)
		if err != nil {
			return fmt.Errorf("failed to set app instance label: %w", err)
		}
		return nil
	case v1alpha1.TrackingMethodAnnotation:
		return setAppInstanceAnnotation()
	case v1alpha1.TrackingMethodAnnotationAndLabel:
		err := setAppInstanceAnnotation()
		if err != nil {
			return err
		}
		if len(val) > LabelMaxLength {
			val = val[:LabelMaxLength]
			// Prevent errors if the truncated name ends in a special character.
			// See https://github.com/argoproj/argo-cd/issues/18237.
			for !OkEndPattern.MatchString(val) {
				if len(val) <= 1 {
					return errors.New("failed to set app instance label: unable to truncate label to not end with a special character")
				}
				val = val[:len(val)-1]
			}
		}
		err = kube.SetAppInstanceLabel(un, key, val)
		if err != nil {
			return fmt.Errorf("failed to set app instance label: %w", err)
		}
		return nil
	default:
		return setAppInstanceAnnotation()
	}
}

// BuildAppInstanceValue build resource tracking id in format <application-name>;<group>/<kind>/<namespace>/<name>
func (rt *resourceTracking) BuildAppInstanceValue(value AppInstanceValue) string {
	return fmt.Sprintf("%s:%s/%s:%s/%s", value.ApplicationName, value.Group, value.Kind, value.Namespace, value.Name)
}

// ParseAppInstanceValue parse resource tracking id from format <application-name>:<group>/<kind>:<namespace>/<name> to struct
func (rt *resourceTracking) ParseAppInstanceValue(value string) (*AppInstanceValue, error) {
	var appInstanceValue AppInstanceValue
	parts := strings.SplitN(value, ":", 3)
	appInstanceValue.ApplicationName = parts[0]
	if len(parts) != 3 {
		return nil, ErrWrongResourceTrackingFormat
	}
	groupParts := strings.Split(parts[1], "/")
	if len(groupParts) != 2 {
		return nil, ErrWrongResourceTrackingFormat
	}
	nsParts := strings.Split(parts[2], "/")
	if len(nsParts) != 2 {
		return nil, ErrWrongResourceTrackingFormat
	}
	appInstanceValue.Group = groupParts[0]
	appInstanceValue.Kind = groupParts[1]
	appInstanceValue.Namespace = nsParts[0]
	appInstanceValue.Name = nsParts[1]
	return &appInstanceValue, nil
}

// Normalize updates live resource and removes diff caused by missing annotation or extra tracking label.
// The normalization is required to ensure smooth transition to new tracking method.
func (rt *resourceTracking) Normalize(config, live *unstructured.Unstructured, labelKey, trackingMethod string) error {
	if IsOldTrackingMethod(trackingMethod) {
		return nil
	}

	if live == nil || config == nil {
		return nil
	}

	label, err := kube.GetAppInstanceLabel(live, labelKey)
	if err != nil {
		return fmt.Errorf("failed to get app instance label: %w", err)
	}
	if label == "" {
		return nil
	}

	if kubeutil.IsCRD(live) {
		// CRDs don't get tracking annotations.
		return nil
	}

	annotation, err := kube.GetAppInstanceAnnotation(config, common.AnnotationKeyAppInstance)
	if err != nil {
		return err
	}
	err = kube.SetAppInstanceAnnotation(live, common.AnnotationKeyAppInstance, annotation)
	if err != nil {
		return err
	}

	label, err = kube.GetAppInstanceLabel(config, labelKey)
	if err != nil {
		return fmt.Errorf("failed to get app instance label: %w", err)
	}
	if label == "" {
		err = kube.RemoveLabel(live, labelKey)
		if err != nil {
			return fmt.Errorf("failed to remove app instance label: %w", err)
		}
	}

	return nil
}
