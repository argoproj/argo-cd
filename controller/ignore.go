package controller

import (
	"github.com/argoproj/argo-cd/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strings"
)

// should we ignore this resource?
func ignore(obj *unstructured.Unstructured) bool {
	// ignore helm hooks, except crd-install
	hooks, ok := obj.GetAnnotations()[common.AnnotationKeyHelmHook]
	return ok && !strings.Contains(hooks, common.AnnotationValueHelmHookCRDInstall)
}
