package resource

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
)

// should we Ignore this resource?
func Ignore(obj *unstructured.Unstructured) bool {
	// Ignore helm hooks, except crd-install
	// Jesse: "we need to pretend that they don’t even exist" ;-)
	_, hasHook := obj.GetAnnotations()[common.AnnotationKeyHook]
	hooks, hasHelmHook := obj.GetAnnotations()[common.AnnotationKeyHelmHook]
	return !hasHook && hasHelmHook && !strings.Contains(hooks, common.AnnotationValueHelmHookCRDInstall)
}
