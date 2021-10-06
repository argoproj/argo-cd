package normalizers

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/kube"

	"github.com/argoproj/argo-cd/v2/util/resource_tracking"
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

	annotation := kube.GetAppInstanceAnnotation(origin, common.AnnotationKeyAppInstance)
	err := kube.SetAppInstanceAnnotation(live, common.AnnotationKeyAppInstance, annotation)
	if err != nil {
		return err
	}
	if config != nil {
		err = kube.SetAppInstanceAnnotation(config, common.AnnotationKeyAppInstance, annotation)
		if err != nil {
			return err
		}
		err = kube.SetAppInstanceLabel(config, common.LabelKeyAppInstance, kube.GetAppInstanceLabel(live, common.LabelKeyAppInstance))
		if err != nil {
			return err
		}
	}

	return nil
}
