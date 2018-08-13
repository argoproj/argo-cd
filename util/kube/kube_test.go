package kube

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"testing"

	"github.com/argoproj/argo-cd/test"
	"github.com/ghodss/yaml"
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

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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

func TestGetCachedServerResources(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(test.DemoService(), test.DemoDeployment())
	fakeDiscovery, ok := kubeclientset.Discovery().(*fakediscovery.FakeDiscovery)
	assert.True(t, ok)
	fakeDiscovery.Fake.Resources = resourceList()
	resList, err := GetCachedServerResources("host", fakeDiscovery)
	count := 0
	for _, resGroup := range resList {
		for range resGroup.APIResources {
			count++
		}
	}
	assert.Nil(t, err)
	assert.Equal(t, 11, count)

	// set resources to empty list and make sure we get the cached result
	fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}
	resList, err = GetCachedServerResources("host", fakeDiscovery)
	count = 0
	for _, resGroup := range resList {
		for range resGroup.APIResources {
			count++
		}
	}
	assert.Nil(t, err)
	assert.Equal(t, 11, count)
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

func TestConvertToVersion(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/nginx.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	// convert an extensions/v1beta1 object into an apps/v1
	newObj, err := ConvertToVersion(&obj, "apps", "v1")
	assert.Nil(t, err)
	gvk := newObj.GroupVersionKind()
	assert.Equal(t, "apps", gvk.Group)
	assert.Equal(t, "v1", gvk.Version)

	// converting it again should not have any affect
	newObj, err = ConvertToVersion(&obj, "apps", "v1")
	assert.Nil(t, err)
	gvk = newObj.GroupVersionKind()
	assert.Equal(t, "apps", gvk.Group)
	assert.Equal(t, "v1", gvk.Version)
}

const depWithoutSelector = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx
        ports:
        - containerPort: 80
`

const depWithSelector = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx
        ports:
        - containerPort: 80
`

func TestUnsetLabels(t *testing.T) {
	for _, yamlStr := range []string{depWithoutSelector, depWithSelector} {
		var obj unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yamlStr), &obj)
		assert.Nil(t, err)

		err = SetLabel(&obj, common.LabelApplicationName, "my-app")
		assert.Nil(t, err)

		err = UnsetLabel(&obj, common.LabelApplicationName)
		assert.Nil(t, err)

		manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
		assert.Nil(t, err)
		log.Println(string(manifestBytes))

		var depV1Beta1 extv1beta1.Deployment
		err = json.Unmarshal(manifestBytes, &depV1Beta1)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(depV1Beta1.Spec.Selector.MatchLabels))
		assert.Equal(t, "nginx", depV1Beta1.Spec.Selector.MatchLabels["app"])
		assert.Equal(t, 1, len(depV1Beta1.Spec.Template.Labels))
		assert.Equal(t, "nginx", depV1Beta1.Spec.Template.Labels["app"])
		assert.Equal(t, "", depV1Beta1.Spec.Template.Labels[common.LabelApplicationName])
	}

}

func TestSetLabels(t *testing.T) {
	for _, yamlStr := range []string{depWithoutSelector, depWithSelector} {
		var obj unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yamlStr), &obj)
		assert.Nil(t, err)

		err = SetLabel(&obj, common.LabelApplicationName, "my-app")
		assert.Nil(t, err)

		manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
		assert.Nil(t, err)
		log.Println(string(manifestBytes))

		var depV1Beta1 extv1beta1.Deployment
		err = json.Unmarshal(manifestBytes, &depV1Beta1)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(depV1Beta1.Spec.Selector.MatchLabels))
		assert.Equal(t, "nginx", depV1Beta1.Spec.Selector.MatchLabels["app"])
		assert.Equal(t, 2, len(depV1Beta1.Spec.Template.Labels))
		assert.Equal(t, "nginx", depV1Beta1.Spec.Template.Labels["app"])
		assert.Equal(t, "my-app", depV1Beta1.Spec.Template.Labels[common.LabelApplicationName])
	}

}

func TestCleanKubectlOutput(t *testing.T) {
	testString := `error: error validating "STDIN": error validating data: ValidationError(Deployment.spec): missing required field "selector" in io.k8s.api.apps.v1beta2.DeploymentSpec; if you choose to ignore these errors, turn validation off with --validate=false`
	assert.Equal(t, cleanKubectlOutput(testString), `error validating data: ValidationError(Deployment.spec): missing required field "selector" in io.k8s.api.apps.v1beta2.DeploymentSpec`)
}
