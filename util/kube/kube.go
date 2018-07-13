// Package kube provides helper utilities common for kubernetes
package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/util/cache"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	listVerb             = "list"
	deleteVerb           = "delete"
	deleteCollectionVerb = "deletecollection"
	watchVerb            = "watch"
)

const (
	ServiceKind     = "Service"
	EndpointsKind   = "Endpoints"
	DeploymentKind  = "Deployment"
	ReplicaSetKind  = "ReplicaSet"
	StatefulSetKind = "StatefulSet"
	DaemonSetKind   = "DaemonSet"
	IngressKind     = "Ingress"
)

const (
	apiResourceCacheDuration = 10 * time.Minute
)

var (
	// location to use for generating temporary files, such as the kubeconfig needed by kubectl
	kubectlTempDir string
	// apiResourceCache is a in-memory cache of api resources supported by a k8s server
	apiResourceCache = cache.NewInMemoryCache(apiResourceCacheDuration)
)

func init() {
	fileInfo, err := os.Stat("/dev/shm")
	if err == nil && fileInfo.IsDir() {
		kubectlTempDir = "/dev/shm"
	}
}

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

// SetLabel sets our app labels against an unstructured object
func SetLabel(target *unstructured.Unstructured, key, val string) error {
	labels := target.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = val
	target.SetLabels(labels)

	// special case for deployment types: make sure that derived replicaset and pod has application label
	switch target.GetKind() {
	case DeploymentKind, ReplicaSetKind, StatefulSetKind, DaemonSetKind:
		templateLabels, ok, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "template", "metadata", "labels")
		if err != nil {
			return err
		}
		if !ok || templateLabels == nil {
			templateLabels = make(map[string]interface{})
		}
		templateLabels[key] = val
		err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "template", "metadata", "labels")
		if err != nil {
			return err
		}
		// The following is a workaround for issue #335. In API version extensions/v1beta1 or
		// apps/v1beta1, if a spec omits spec.selector then k8s will default the
		// spec.selector.matchLabels to match spec.template.metadata.labels. This means ArgoCD
		// labels can potentially make their way into spec.selector.matchLabels, which is a bad
		// thing. The following logic prevents this behavior.
		switch target.GetAPIVersion() {
		case "apps/v1beta1", "extensions/v1beta1":
			selector, _, err := unstructured.NestedMap(target.UnstructuredContent(), "spec", "selector")
			if err != nil {
				return err
			}
			if len(selector) == 0 {
				// If we get here, user did not set spec.selector in their manifest. We do not want
				// our ArgoCD labels to get defaulted by kubernetes, so we explicitly set the labels
				// for them (minus the ArgoCD labels).
				delete(templateLabels, key)
				err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "selector", "matchLabels")
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// GetCachedServerResources discovers API resources supported by a Kube API server.
// Caches the results for apiResourceCacheDuration (per host)
func GetCachedServerResources(host string, disco discovery.DiscoveryInterface) ([]*metav1.APIResourceList, error) {
	var resList []*metav1.APIResourceList
	cacheKey := fmt.Sprintf("apires|%s", host)
	err := apiResourceCache.Get(cacheKey, &resList)
	if err == nil {
		log.Debugf("cache hit: %s", cacheKey)
		return resList, nil
	}
	if err == cache.ErrCacheMiss {
		log.Infof("cache miss: %s", cacheKey)
	} else {
		log.Warnf("cache error %s: %v", cacheKey, err)
	}
	resList, err = disco.ServerResources()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	err = apiResourceCache.Set(&cache.Item{
		Key:    cacheKey,
		Object: resList,
	})
	if err != nil {
		log.Warnf("Failed to cache %s: %v", cacheKey, err)
	}
	return resList, nil
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

func WatchResourcesWithLabel(ctx context.Context, config *rest.Config, namespace string, labelName string) (chan watch.Event, error) {
	log.Infof("Start watching for resources changes with label %s in cluster %s", labelName, config.Host)
	dynClientPool := dynamic.NewDynamicClientPool(config)
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	serverResources, err := GetCachedServerResources(config.Host, disco)
	if err != nil {
		return nil, err
	}

	resources := make([]dynamic.ResourceInterface, 0)
	for _, apiResourcesList := range serverResources {
		for i := range apiResourcesList.APIResources {
			apiResource := apiResourcesList.APIResources[i]
			watchSupported := false
			for _, verb := range apiResource.Verbs {
				if verb == watchVerb {
					watchSupported = true
					break
				}
			}
			if watchSupported {
				dclient, err := dynClientPool.ClientForGroupVersionKind(schema.FromAPIVersionAndKind(apiResourcesList.GroupVersion, apiResource.Kind))
				if err != nil {
					return nil, err
				}
				resources = append(resources, dclient.Resource(&apiResource, namespace))
			}
		}
	}
	ch := make(chan watch.Event)
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(resources))
		for i := 0; i < len(resources); i++ {
			resource := resources[i]
			go func() {
				defer wg.Done()
				watch, err := resource.Watch(metav1.ListOptions{LabelSelector: labelName})
				go func() {
					select {
					case <-ctx.Done():
						watch.Stop()
					}
				}()
				if err == nil {
					for event := range watch.ResultChan() {
						ch <- event
					}
				}
			}()
		}
		wg.Wait()
		close(ch)
		log.Infof("Stop watching for resources changes with label %s in cluster %s", labelName, config.ServerName)
	}()
	return ch, nil
}

// GetResourcesWithLabel returns all kubernetes resources with specified label
func GetResourcesWithLabel(config *rest.Config, namespace string, labelName string, labelValue string) ([]*unstructured.Unstructured, error) {
	dynClientPool := dynamic.NewDynamicClientPool(config)
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	resources, err := GetCachedServerResources(config.Host, disco)
	if err != nil {
		return nil, err
	}

	var resourceInterfaces []dynamic.ResourceInterface

	for _, apiResourcesList := range resources {
		for i := range apiResourcesList.APIResources {
			apiResource := apiResourcesList.APIResources[i]
			listSupported := false
			for _, verb := range apiResource.Verbs {
				if verb == listVerb {
					listSupported = true
					break
				}
			}
			if listSupported {
				dclient, err := dynClientPool.ClientForGroupVersionKind(schema.FromAPIVersionAndKind(apiResourcesList.GroupVersion, apiResource.Kind))
				if err != nil {
					return nil, err
				}
				resourceInterfaces = append(resourceInterfaces, dclient.Resource(&apiResource, namespace))
			}
		}
	}

	var asyncErr error
	var result []*unstructured.Unstructured

	var wg sync.WaitGroup
	wg.Add(len(resourceInterfaces))
	for i := range resourceInterfaces {
		client := resourceInterfaces[i]
		go func() {
			defer wg.Done()
			list, err := client.List(metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", labelName, labelValue),
			})
			if err != nil {
				asyncErr = err
				return
			}
			// apply client side filtering since not every kubernetes API supports label filtering
			for i := range list.(*unstructured.UnstructuredList).Items {
				item := list.(*unstructured.UnstructuredList).Items[i]
				labels := item.GetLabels()
				if labels != nil {
					if value, ok := labels[labelName]; ok && value == labelValue {
						result = append(result, &item)
					}
				}
			}
		}()
	}
	wg.Wait()
	return result, asyncErr
}

// DeleteResourceWithLabel delete all resources which match to specified label selector
func DeleteResourceWithLabel(config *rest.Config, namespace string, labelName string, labelValue string) error {
	dynClientPool := dynamic.NewDynamicClientPool(config)
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	resources, err := GetCachedServerResources(config.Host, disco)
	if err != nil {
		return err
	}

	var resourceInterfaces []struct {
		dynamic.ResourceInterface
		bool
	}

	for _, apiResourcesList := range resources {
		for i := range apiResourcesList.APIResources {
			apiResource := apiResourcesList.APIResources[i]
			deleteCollectionSupported := false
			deleteSupported := false
			for _, verb := range apiResource.Verbs {
				if verb == deleteCollectionVerb {
					deleteCollectionSupported = true
				} else if verb == deleteVerb {
					deleteSupported = true
				}
			}
			dclient, err := dynClientPool.ClientForGroupVersionKind(schema.FromAPIVersionAndKind(apiResourcesList.GroupVersion, apiResource.Kind))
			if err != nil {
				return err
			}

			if deleteCollectionSupported || deleteSupported {
				resourceInterfaces = append(resourceInterfaces, struct {
					dynamic.ResourceInterface
					bool
				}{dclient.Resource(&apiResource, namespace), deleteCollectionSupported})
			}
		}
	}

	var asyncErr error
	propagationPolicy := metav1.DeletePropagationForeground

	var wg sync.WaitGroup
	wg.Add(len(resourceInterfaces))

	for i := range resourceInterfaces {
		client := resourceInterfaces[i].ResourceInterface
		deleteCollectionSupported := resourceInterfaces[i].bool

		go func() {
			defer wg.Done()
			if deleteCollectionSupported {
				err = client.DeleteCollection(&metav1.DeleteOptions{
					PropagationPolicy: &propagationPolicy,
				}, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", labelName, labelValue)})
				if err != nil && !apierr.IsNotFound(err) {
					asyncErr = err
				}
			} else {
				items, err := client.List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", labelName, labelValue)})
				if err != nil {
					asyncErr = err
					return
				}
				for _, item := range items.(*unstructured.UnstructuredList).Items {
					// apply client side filtering since not every kubernetes API supports label filtering
					labels := item.GetLabels()
					if labels != nil {
						if value, ok := labels[labelName]; ok && value == labelValue {
							err = client.Delete(item.GetName(), &metav1.DeleteOptions{
								PropagationPolicy: &propagationPolicy,
							})
							if err != nil && !apierr.IsNotFound(err) {
								asyncErr = err
								return
							}
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	return asyncErr
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

// deleteFile is best effort deletion of a file
func deleteFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}
	_ = os.Remove(path)
}

// DeleteResource deletes resource
func DeleteResource(config *rest.Config, obj *unstructured.Unstructured, namespace string) error {
	dynClientPool := dynamic.NewDynamicClientPool(config)
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	gvk := obj.GroupVersionKind()
	dclient, err := dynClientPool.ClientForGroupVersionKind(gvk)
	if err != nil {
		return err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return err
	}
	reIf := dclient.Resource(apiResource, namespace)
	propagationPolicy := metav1.DeletePropagationForeground
	return reIf.Delete(obj.GetName(), &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}

// ApplyResource performs an apply of a unstructured resource
func ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force bool) (string, error) {
	log.Infof("Applying resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), config.Host, namespace)
	f, err := ioutil.TempFile(kubectlTempDir, "")
	if err != nil {
		return "", fmt.Errorf("Failed to generate temp file for kubeconfig: %v", err)
	}
	_ = f.Close()
	err = WriteKubeConfig(config, namespace, f.Name())
	if err != nil {
		return "", fmt.Errorf("Failed to write kubeconfig: %v", err)
	}
	defer deleteFile(f.Name())
	manifestBytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	applyArgs := []string{"--kubeconfig", f.Name(), "-n", namespace, "apply", "-f", "-"}
	if dryRun {
		applyArgs = append(applyArgs, "--dry-run")
	}
	if force {
		applyArgs = append(applyArgs, "--force")
	}
	cmd := exec.Command("kubectl", applyArgs...)
	log.Info(cmd.Args)
	cmd.Stdin = bytes.NewReader(manifestBytes)
	out, err := cmd.Output()
	if err != nil {
		if exErr, ok := err.(*exec.ExitError); ok {
			errMsg := cleanKubectlOutput(string(exErr.Stderr))
			return "", errors.New(errMsg)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// cleanKubectlOutput makes the error output of kubectl a little better to read
func cleanKubectlOutput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Replace(s, ": error validating \"STDIN\"", "", -1)
	s = strings.Replace(s, ": error when creating \"STDIN\"", "", -1)
	s = strings.Replace(s, "; if you choose to ignore these errors, turn validation off with --validate=false", "", -1)
	s = strings.Replace(s, "error: error", "error", -1)
	return s
}

// WriteKubeConfig takes a rest.Config and writes it as a kubeconfig at the specified path
func WriteKubeConfig(restConfig *rest.Config, namespace, filename string) error {
	var kubeConfig = clientcmdapi.Config{
		CurrentContext: restConfig.Host,
		Contexts: map[string]*clientcmdapi.Context{
			restConfig.Host: {
				Cluster:   restConfig.Host,
				AuthInfo:  restConfig.Host,
				Namespace: namespace,
			},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			restConfig.Host: {
				Server: restConfig.Host,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			restConfig.Host: {},
		},
	}
	// Set Cluster info
	if restConfig.TLSClientConfig.Insecure {
		kubeConfig.Clusters[restConfig.Host].InsecureSkipTLSVerify = true
	}
	if restConfig.TLSClientConfig.CAFile != "" {
		kubeConfig.Clusters[restConfig.Host].CertificateAuthority = restConfig.TLSClientConfig.CAFile
	}
	// Set AuthInfo
	if len(restConfig.TLSClientConfig.CAData) > 0 {
		kubeConfig.Clusters[restConfig.Host].CertificateAuthorityData = restConfig.TLSClientConfig.CAData
	}
	if restConfig.TLSClientConfig.CertFile != "" {
		kubeConfig.AuthInfos[restConfig.Host].ClientCertificate = restConfig.TLSClientConfig.CertFile
	}
	if len(restConfig.TLSClientConfig.CertData) > 0 {
		kubeConfig.AuthInfos[restConfig.Host].ClientCertificateData = restConfig.TLSClientConfig.CertData
	}
	if restConfig.TLSClientConfig.KeyFile != "" {
		kubeConfig.AuthInfos[restConfig.Host].ClientKey = restConfig.TLSClientConfig.KeyFile
	}
	if len(restConfig.TLSClientConfig.KeyData) > 0 {
		kubeConfig.AuthInfos[restConfig.Host].ClientKeyData = restConfig.TLSClientConfig.KeyData
	}
	if restConfig.Username != "" {
		kubeConfig.AuthInfos[restConfig.Host].Username = restConfig.Username
	}
	if restConfig.Password != "" {
		kubeConfig.AuthInfos[restConfig.Host].Password = restConfig.Password
	}
	if restConfig.BearerToken != "" {
		kubeConfig.AuthInfos[restConfig.Host].Token = restConfig.BearerToken
	}
	return clientcmd.WriteToFile(kubeConfig, filename)
}

// ConvertToVersion converts an unstructured object into the specified group/version
func ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error) {
	gvk := obj.GroupVersionKind()
	if gvk.Group == group && gvk.Version == version {
		return obj.DeepCopy(), nil
	}
	manifestBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	outputVersion := fmt.Sprintf("%s/%s", group, version)
	cmd := exec.Command("kubectl", "convert", "--output-version", outputVersion, "-o", "json", "--local=true", "-f", "-")
	cmd.Stdin = bytes.NewReader(manifestBytes)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	// NOTE: when kubectl convert runs against stdin (i.e. kubectl convert -f -), the output is
	// a unstructured list instead of an unstructured object
	var objList unstructured.UnstructuredList
	err = json.Unmarshal(out, &objList)
	if err != nil {
		return nil, err
	}
	if len(objList.Items) != 1 {
		return nil, fmt.Errorf("kubectl convert produced unexpected list size: %d", len(objList.Items))
	}
	return &objList.Items[0], nil
}
