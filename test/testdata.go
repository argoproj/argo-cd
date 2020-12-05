package test

import (
	"github.com/argoproj/gitops-engine/pkg/utils/testing"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/common"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	applister "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
)

const (
	FakeArgoCDNamespace = "fake-argocd-ns"
	FakeDestNamespace   = "fake-dest-ns"
	FakeClusterURL      = "https://fake-cluster:443"
)

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
