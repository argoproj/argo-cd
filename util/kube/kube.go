// Package kube provides helper utilities common for kubernetes

package kube

import (
	"encoding/json"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClientConfig return rest config, if path not specified, assume in cluster config
func GetClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

type listResult struct {
	Items []*unstructured.Unstructured `json:"items"`
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
