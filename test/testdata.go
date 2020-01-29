package test

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/common"
	synccommon "github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
	"github.com/argoproj/argo-cd/engine/pkg/utils/testing"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	applister "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
)

const (
	FakeArgoCDNamespace = "fake-argocd-ns"
	FakeDestNamespace   = "fake-dest-ns"
	FakeClusterURL      = "https://fake-cluster:443"
)

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
	return testing.Unstructured(PodManifest)
}

func NewControllerRevision() *unstructured.Unstructured {
	return testing.Unstructured(`
kind: ControllerRevision
apiVersion: metacontroller.k8s.io/v1alpha1
metadata:
  labels:
    app: nginx
    controller.kubernetes.io/hash: c7cd8d57f
  name: web-c7cd8d57f
  namespace: statefulset
revision: 2
`)
}

func NewCRD() *unstructured.Unstructured {
	return testing.Unstructured(`apiVersion: apiextensions.k8s.io/v1beta1
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

// DEPRECATED
// use `Hook(NewPod())` or similar instead
func NewHook(hookType synccommon.HookType) *unstructured.Unstructured {
	return Hook(NewPod(), hookType)
}

func Hook(obj *unstructured.Unstructured, hookType synccommon.HookType) *unstructured.Unstructured {
	return Annotate(obj, "argocd.argoproj.io/hook", string(hookType))
}

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
	return testing.Unstructured(ServiceManifest)
}

var DeploymentManifest = `
{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "nginx-deployment",
    "labels": {
      "app": "nginx"
    }
  },
  "spec": {
    "replicas": 3,
    "selector": {
      "matchLabels": {
        "app": "nginx"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "nginx"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "nginx",
            "image": "nginx:1.15.4",
            "ports": [
              {
                "containerPort": 80
              }
            ]
          }
        ]
      }
    }
  }
}
`

func NewDeployment() *unstructured.Unstructured {
	return testing.Unstructured(DeploymentManifest)
}

func NewFakeConfigMap() *apiv1.ConfigMap {
	cm := apiv1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: FakeArgoCDNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: make(map[string]string),
	}
	return &cm
}

func NewFakeSecret(policy ...string) *apiv1.Secret {
	secret := apiv1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: FakeArgoCDNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	}
	return &secret
}

func NewFakeProjLister(objects ...runtime.Object) applister.AppProjectNamespaceLister {
	fakeAppClientset := apps.NewSimpleClientset(objects...)
	factory := appinformer.NewFilteredSharedInformerFactory(fakeAppClientset, 0, "", func(options *metav1.ListOptions) {})
	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	cancel := StartInformer(projInformer)
	defer cancel()
	return factory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(FakeArgoCDNamespace)
}
