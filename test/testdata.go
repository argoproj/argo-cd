package test

import (
	"context"

	"github.com/alicebob/miniredis/v2"
	"github.com/argoproj/gitops-engine/pkg/utils/testing"
	"github.com/redis/go-redis/v9"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appclient "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/typed/application/v1alpha1"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	applister "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
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

var ConfigMapManifest = `
{	
  "apiVersion": "v1",
  "kind": "ConfigMap",
  "metadata": {
    "name": "my-configmap",
  },
  "data": {
    "config.yaml": "auth: token\nconfig:field"
  }
}`

func NewConfigMap() *unstructured.Unstructured {
	return testing.Unstructured(ConfigMapManifest)
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

type interfaceLister struct {
	appProjects appclient.AppProjectInterface
}

func (l interfaceLister) List(selector labels.Selector) ([]*v1alpha1.AppProject, error) {
	res, err := l.appProjects.List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	items := make([]*v1alpha1.AppProject, len(res.Items))
	for i := range res.Items {
		items[i] = &res.Items[i]
	}
	return items, nil
}

func (l interfaceLister) Get(name string) (*v1alpha1.AppProject, error) {
	return l.appProjects.Get(context.Background(), name, metav1.GetOptions{})
}

func NewFakeProjListerFromInterface(appProjects appclient.AppProjectInterface) applister.AppProjectNamespaceLister {
	return &interfaceLister{appProjects: appProjects}
}

func NewFakeProjLister(objects ...runtime.Object) applister.AppProjectNamespaceLister {
	fakeAppClientset := apps.NewSimpleClientset(objects...)
	factory := appinformer.NewSharedInformerFactoryWithOptions(fakeAppClientset, 0, appinformer.WithNamespace(""), appinformer.WithTweakListOptions(func(options *metav1.ListOptions) {}))
	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	cancel := StartInformer(projInformer)
	defer cancel()
	return factory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(FakeArgoCDNamespace)
}

func NewInMemoryRedis() (*redis.Client, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr.Close
}
