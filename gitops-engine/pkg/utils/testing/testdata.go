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
	return Unstructured(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: testcrds.argoproj.io
spec:
  group: test.io
  version: v1
  scope: Namespaced
  names:
    plural: testcrds
    kind: TestCrd`)
}

func NewNamespace() *unstructured.Unstructured {
	return Unstructured(`apiVersion: v1
kind: Namespace
metadata:
  name: testnamespace
spec:`)
}

func NewRole() *unstructured.Unstructured {
	return Unstructured(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "watch", "list"]`)
}

func NewRoleBinding() *unstructured.Unstructured {
	return Unstructured(`apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: my-role-binding
subjects:
- kind: User
  name: user
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: my-role
  apiGroup: rbac.authorization.k8s.io`)
}

func NewClusterRole() *unstructured.Unstructured {
	return Unstructured(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-cluster-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "watch", "list"]`)
}

func NewClusterRoleBinding() *unstructured.Unstructured {
	return Unstructured(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-cluster-role-binding
subjects:
- kind: Group
  name: group
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: my-cluster-role
  apiGroup: rbac.authorization.k8s.io`)
}
