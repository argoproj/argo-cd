package kube

import (
	"encoding/json"
	"log"
	"testing"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
	"github.com/stretchr/testify/assert"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

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
	kubeclientset := fake.NewSimpleClientset(test.DemoService(), test.DemoDeployment())
	fakeDiscovery, ok := kubeclientset.Discovery().(*fakediscovery.FakeDiscovery)
	assert.True(t, ok)
	fakeDiscovery.Fake.Resources = resourceList()
	apiRes, err := ListAPIResources(fakeDiscovery)
	assert.Nil(t, err)
	assert.Equal(t, 11, len(apiRes))
}

func TestGetLiveResource(t *testing.T) {
	demoSvc := test.DemoService()
	kubeclientset := fake.NewSimpleClientset(demoSvc, test.DemoDeployment())
	fakeDiscovery, ok := kubeclientset.Discovery().(*fakediscovery.FakeDiscovery)
	assert.True(t, ok)
	fakeDiscovery.Fake.Resources = resourceList()

	fakeDynClient := fakedynamic.FakeClient{
		Fake: &kubetesting.Fake{},
	}
	fakeDynClient.Fake.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		svc, err := kubeclientset.CoreV1().Services(test.TestNamespace).Get(demoSvc.Name, metav1.GetOptions{})
		assert.Nil(t, err)
		svc.Kind = "Service"
		return true, MustToUnstructured(svc), nil
	})

	uObj := MustToUnstructured(test.DemoService())
	fakeAPIResource := metav1.APIResource{}
	liveObj, err := GetLiveResource(&fakeDynClient, uObj, &fakeAPIResource, test.TestNamespace)
	assert.Nil(t, err)
	assert.Equal(t, uObj.GetName(), liveObj.GetName())
}

func TestListResources(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(test.DemoService(), test.DemoDeployment())
	fakeDynClient := fakedynamic.FakeClient{
		Fake: &kubetesting.Fake{},
	}
	fakeDynClient.Fake.AddReactor("list", "services", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		svcList, err := kubeclientset.CoreV1().Services(test.TestNamespace).List(metav1.ListOptions{})
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
	resList, err := ListResources(&fakeDynClient, apiResource, test.TestNamespace, metav1.ListOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(resList))
}
