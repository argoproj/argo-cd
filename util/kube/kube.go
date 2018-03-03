// Package kube provides helper utilities common for kubernetes

package kube

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// TestConfig tests to make sure the REST config is usable
func TestConfig(config *rest.Config) error {
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("REST config invalid: %s", err)
	}
	_, err = kubeclientset.ServerVersion()
	if err != nil {
		return fmt.Errorf("REST config invalid: %s", err)
	}
	return nil
}

// ToUnstructured converts a concrete K8s API type to a un unstructured object
func ToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	uObj, err := runtime.NewTestUnstructuredConverter(equality.Semantic).ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: uObj}, nil
}

// MustToUnstructured converts a concrete K8s API type to a un unstructured object and panics if not successful
func MustToUnstructured(obj interface{}) *unstructured.Unstructured {
	uObj, err := ToUnstructured(obj)
	if err != nil {
		panic(err)
	}
	return uObj
}

// ListAPIResources discovers all API resources supported by the Kube API sererver
func ListAPIResources(disco discovery.DiscoveryInterface) ([]metav1.APIResource, error) {
	apiResources := make([]metav1.APIResource, 0)
	resList, err := disco.ServerResources()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, resGroup := range resList {
		for _, apiRes := range resGroup.APIResources {
			apiResources = append(apiResources, apiRes)
		}
	}
	return apiResources, nil
}

// GetLiveResource returns the corresponding live resource from a unstructured object
func GetLiveResource(dclient dynamic.Interface, obj *unstructured.Unstructured, apiResource *metav1.APIResource, namespace string) (*unstructured.Unstructured, error) {
	resourceName := obj.GetName()
	if resourceName == "" {
		return nil, fmt.Errorf("resource was supplied without a name")
	}
	reIf := dclient.Resource(apiResource, namespace)
	liveObj, err := reIf.Get(resourceName, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			log.Infof("No live counterpart to %s/%s/%s/%s in namespace: '%s'", apiResource.Group, apiResource.Version, apiResource.Name, resourceName, namespace)
			return nil, nil
		}
		return nil, errors.WithStack(err)
	}
	return liveObj, nil
}

// GetLiveResources returns the corresponding live resource from a list of resources
func GetLiveResources(config *rest.Config, objs []*unstructured.Unstructured, namespace string) ([]*unstructured.Unstructured, error) {
	liveObjs := make([]*unstructured.Unstructured, len(objs))
	dynClientPool := dynamic.NewDynamicClientPool(config)
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	for i, obj := range objs {
		gvk := obj.GroupVersionKind()
		dclient, err := dynClientPool.ClientForGroupVersionKind(gvk)
		if err != nil {
			return nil, err
		}
		apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
		if err != nil {
			return nil, err
		}
		liveObj, err := GetLiveResource(dclient, obj, apiResource, namespace)
		if err != nil {
			return nil, err
		}
		liveObjs[i] = liveObj
	}
	return liveObjs, nil
}

// See: https://github.com/ksonnet/ksonnet/blob/master/utils/client.go
func ServerResourceForGroupVersionKind(disco discovery.DiscoveryInterface, gvk schema.GroupVersionKind) (*metav1.APIResource, error) {
	resources, err := disco.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return nil, err
	}
	for _, r := range resources.APIResources {
		if r.Kind == gvk.Kind {
			log.Debugf("Chose API '%s' for %s", r.Name, gvk)
			return &r, nil
		}
	}
	return nil, fmt.Errorf("Server is unable to handle %s", gvk)
}

type listResult struct {
	Items []*unstructured.Unstructured `json:"items"`
}

// ListResources returns a list of resources of a particular API type using the dynamic client
func ListResources(dclient dynamic.Interface, apiResource metav1.APIResource, namespace string, listOpts metav1.ListOptions) ([]*unstructured.Unstructured, error) {
	reIf := dclient.Resource(&apiResource, namespace)
	liveObjs, err := reIf.List(listOpts)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	liveObjsBytes, err := json.Marshal(liveObjs)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var objList listResult
	err = json.Unmarshal(liveObjsBytes, &objList)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return objList.Items, nil
}

// ListAllResources iterates the list of API resources, and returns all resources with the given filters
func ListAllResources(config *rest.Config, apiResources []metav1.APIResource, namespace string, listOpts metav1.ListOptions) ([]*unstructured.Unstructured, error) {
	// itemMap dedups items when there is duplication of a resource in multiple API types
	// e.g. extensions/v1beta1/namespaces/default/deployments and apps/v1/namespaces/default/deployments
	itemMap := make(map[string]*unstructured.Unstructured)

	for _, apiResource := range apiResources {
		dynConfig := *config
		dynConfig.GroupVersion = &schema.GroupVersion{
			Group:   apiResource.Group,
			Version: apiResource.Kind,
		}
		dclient, err := dynamic.NewClient(&dynConfig)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		resList, err := ListResources(dclient, apiResource, namespace, listOpts)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		for _, liveObj := range resList {
			itemMap[string(liveObj.GetUID())] = liveObj
		}

	}
	resources := make([]*unstructured.Unstructured, len(itemMap))
	i := 0
	for _, obj := range itemMap {
		resources[i] = obj
		i++
	}
	return resources, nil
}

// ApplyResource performs an apply of a unstructured resource
func ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string) (*unstructured.Unstructured, error) {
	dynClientPool := dynamic.NewDynamicClientPool(config)
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	gvk := obj.GroupVersionKind()
	dclient, err := dynClientPool.ClientForGroupVersionKind(gvk)
	if err != nil {
		return nil, err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return nil, err
	}
	reIf := dclient.Resource(apiResource, namespace)
	liveObj, err := reIf.Update(obj)
	if err != nil {
		if !apierr.IsNotFound(err) {
			return nil, errors.WithStack(err)
		}
		liveObj, err = reIf.Create(obj)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return liveObj, nil
}
