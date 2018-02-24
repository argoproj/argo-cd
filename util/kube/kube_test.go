package kube

import (
	"encoding/json"
	"log"
	"testing"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

const (
	testAppName         = "test-app"
	testNamespace       = "test-namespace"
	testEnvName         = "test-env"
	testAppInstanceName = "test-app-instance"
)

func demoService() *apiv1.Service {
	return &apiv1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeyAppInstance: testAppInstanceName,
			},
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
			Selector: map[string]string{
				"app": "demo",
			},
			Type: "ClusterIP",
		},
	}

}

func demoDeployment() *appsv1.Deployment {
	var two int32 = 2
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeyAppInstance: testAppInstanceName,
			},
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

func resourceList() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{
			GroupVersion: apiv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "pods", Namespaced: true, Kind: "Pod"},
				{Name: "services", Namespaced: true, Kind: "Service"},
				{Name: "replicationcontrollers", Namespaced: true, Kind: "ReplicationController"},
				{Name: "replicationcontrollers/scale", Namespaced: true, Kind: "Scale", Group: "autoscaling", Version: "v1"},
			},
		},
		{
			GroupVersion: extv1beta1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet"},
				{Name: "replicasets/scale", Namespaced: true, Kind: "Scale"},
			},
		},
		{
			GroupVersion: appsv1beta2.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
				{Name: "deployments/scale", Namespaced: true, Kind: "Scale", Group: "apps", Version: "v1beta2"},
			},
		},
		{
			GroupVersion: appsv1beta1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
				{Name: "statefulsets/scale", Namespaced: true, Kind: "Scale", Group: "apps", Version: "v1beta1"},
			},
		},
		{
			GroupVersion: argoappv1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "applications", Namespaced: true, Kind: "Application"},
			},
		},
	}
}

func TestListAPIResources(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(demoService(), demoDeployment())
	fakeDiscovery, ok := kubeclientset.Discovery().(*fakediscovery.FakeDiscovery)
	assert.True(t, ok)
	fakeDiscovery.Fake.Resources = resourceList()
	apiRes, err := ListAPIResources(fakeDiscovery)
	assert.Nil(t, err)
	assert.Equal(t, 11, len(apiRes))
}

func TestListResources(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(demoService(), demoDeployment())
	fakeDynClient := fakedynamic.FakeClient{
		Fake: &kubetesting.Fake{},
	}
	fakeDynClient.Fake.AddReactor("list", "services", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		svcList, err := kubeclientset.CoreV1().Services(testNamespace).List(metav1.ListOptions{})
		assert.Nil(t, err)
		svcList.Kind = "ServiceList"
		svcListBytes, err := json.Marshal(svcList)
		log.Println(string(svcListBytes))
		assert.Nil(t, err)
		var uList unstructured.UnstructuredList
		err = json.Unmarshal(svcListBytes, &uList)
		assert.Nil(t, err)
		return true, &uList, nil
	})

	apiResource := metav1.APIResource{
		Name:       "services",
		Namespaced: true,
		Version:    "v1",
		Kind:       "Service",
	}
	resList, err := ListResources(&fakeDynClient, apiResource, testNamespace, metav1.ListOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(resList))
}
