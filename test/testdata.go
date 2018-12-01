package test

import (
	"encoding/json"

	"github.com/gobuffalo/packr"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/errors"
)

const (
	FakeArgoCDNamespace = "fake-argocd-ns"
	FakeDestNamespace   = "fake-dest-ns"
	FakeClusterURL      = "https://fake-cluster:443"
	TestAppInstanceName = "test-app-instance"
)

var (
	box           = packr.NewBox("../util/rbac")
	BuiltinPolicy string
)

func init() {
	var err error
	BuiltinPolicy, err = box.MustString("builtin-policy.csv")
	errors.CheckError(err)
}

var PodManifest = []byte(`
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
`)

func NewPod() *unstructured.Unstructured {
	var un unstructured.Unstructured
	err := json.Unmarshal(PodManifest, &un)
	if err != nil {
		panic(err)
	}
	return &un
}

var ServiceManifest = []byte(`
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
`)

func NewService() *unstructured.Unstructured {
	var un unstructured.Unstructured
	err := json.Unmarshal(ServiceManifest, &un)
	if err != nil {
		panic(err)
	}
	return &un
}

var DeploymentManifest = []byte(`
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
`)

func NewDeployment() *unstructured.Unstructured {
	var un unstructured.Unstructured
	err := json.Unmarshal(DeploymentManifest, &un)
	if err != nil {
		panic(err)
	}
	return &un
}

func DemoDeployment() *appsv1.Deployment {
	var two int32 = 2
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: FakeArgoCDNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &two,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "demo",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "demo",
							Image: "gcr.io/kuar-demo/kuard-amd64:1",
							Ports: []apiv1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}
