package testing

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	FakeArgoCDNamespace = "fake-argocd-ns"
)

func HelmHook(obj *unstructured.Unstructured, hookType string) *unstructured.Unstructured {
	return Annotate(obj, "helm.sh/hook", hookType)
}

func Annotate(obj *unstructured.Unstructured, key, val string) *unstructured.Unstructured {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = val
	obj.SetAnnotations(annotations)
	return obj
}

var PodManifest = `
{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "my-pod"
  },
  "spec": {
    "containers": [
      {
        "image": "nginx:1.7.9",
        "name": "nginx",
        "resources": {
          "requests": {
            "cpu": 0.2
          }
        }
      }
    ]
  }
}
`

func NewPod() *unstructured.Unstructured {
	return Unstructured(PodManifest)
}

var ServiceManifest = `
{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
    "name": "my-service"
  },
  "spec": {
    "ports": [
      {
        "name": "http",
        "protocol": "TCP",
        "port": 80,
        "targetPort": 8080
      }
    ],
    "selector": {
      "app": "my-service"
    }
  }
}
`

func NewService() *unstructured.Unstructured {
	return Unstructured(ServiceManifest)
}

func NewCRD() *unstructured.Unstructured {
	return Unstructured(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: testcrds.argoproj.io
spec:
  group: argoproj.io
  version: v1
  scope: Namespaced
  names:
    plural: testcrds
    kind: TestCrd`)
}
