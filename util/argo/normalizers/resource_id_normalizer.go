package normalizers

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/util/resource_tracking"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/kube"
)

type resourceIdNormalizer struct {
	trackingMethod string
}

func init() {

}

// NewResourceIdNormalizer
func NewResourceIdNormalizer(trackingMethod string) (*resourceIdNormalizer, error) {
	normalizer := resourceIdNormalizer{trackingMethod: trackingMethod}
	return &normalizer, nil
}

// Normalize re-format custom resource fields using built-in Kubernetes types JSON marshaler.
// This technique allows avoiding false drift detections in CRDs that import data structures from Kubernetes codebase.
func (n *resourceIdNormalizer) Normalize(origin, config, live *unstructured.Unstructured) error {
	if resource_tracking.IsOldTrackingMethod(n.trackingMethod) {
		return nil
	}

	if live == nil {
		return nil
	}

	if n.trackingMethod == string(resource_tracking.TrackingMethodAnnotation) {
		annotation := kube.GetAppInstanceAnnotation(origin, common.AnnotationKeyAppInstance)
		_ = kube.SetAppInstanceAnnotation(live, common.AnnotationKeyAppInstance, annotation)
		if config != nil {
			_ = kube.SetAppInstanceAnnotation(config, common.AnnotationKeyAppInstance, annotation)
			_ = kube.SetAppInstanceLabel(config, common.LabelKeyAppInstance, kube.GetAppInstanceLabel(live, common.LabelKeyAppInstance))
		}
	}

	if n.trackingMethod == string(resource_tracking.TrackingMethodAnnotationAndLabel) {
		annotation := kube.GetAppInstanceAnnotation(origin, common.AnnotationKeyAppInstance)
		_ = kube.SetAppInstanceAnnotation(live, common.AnnotationKeyAppInstance, annotation)
		if config != nil {
			_ = kube.SetAppInstanceAnnotation(config, common.AnnotationKeyAppInstance, annotation)
		}
	}

	//
	//label := kube.GetAppInstanceLabel(origin, common.LabelKeyAppInstance)
	//
	//lannotation := kube.GetAppInstanceAnnotation(live, common.AnnotationKeyAppInstance)
	//llabel := kube.GetAppInstanceLabel(live, common.LabelKeyAppInstance)
	//
	//if annotation != "" {
	//}
	//
	//if label != "" {
	//	_ = kube.SetAppInstanceLabel(live, common.LabelKeyAppInstance, label)
	//}
	//
	//if lannotation != "" {
	//	_ = kube.SetAppInstanceAnnotation(origin, common.AnnotationKeyAppInstance, lannotation)
	//}
	//
	//if llabel != "" {
	//	_ = kube.SetAppInstanceLabel(origin, common.LabelKeyAppInstance, llabel)
	//}
	return nil
}
