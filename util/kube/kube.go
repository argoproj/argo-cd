// Package kube provides helper utilities common for kubernetes
package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/util"

	"github.com/ghodss/yaml"
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
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/argoproj/argo-cd/util/cache"
	jsonutil "github.com/argoproj/argo-cd/util/json"
)

const (
	listVerb             = "list"
	deleteVerb           = "delete"
	deleteCollectionVerb = "deletecollection"
	watchVerb            = "watch"
)

const (
	SecretKind                   = "Secret"
	ServiceKind                  = "Service"
	EndpointsKind                = "Endpoints"
	DeploymentKind               = "Deployment"
	ReplicaSetKind               = "ReplicaSet"
	StatefulSetKind              = "StatefulSet"
	DaemonSetKind                = "DaemonSet"
	IngressKind                  = "Ingress"
	JobKind                      = "Job"
	PersistentVolumeClaimKind    = "PersistentVolumeClaim"
	CustomResourceDefinitionKind = "CustomResourceDefinition"
	PodKind                      = "Pod"
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

func FormatResourceKey(group string, kind string, namespace string, name string) string {
	if group == "extensions" {
		group = "apps"
	}

	return fmt.Sprintf("%s:%s:%s:%s", group, kind, namespace, name)
}

func GetResourceKey(obj *unstructured.Unstructured) string {
	return GetResourceKeyNS(obj, "")
}

func GetResourceKeyNS(obj *unstructured.Unstructured, namespace string) string {
	gvk := obj.GroupVersionKind()
	return FormatResourceKey(gvk.Group, gvk.Kind, util.FirstNonEmpty(obj.GetNamespace(), namespace), obj.GetName())
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

// UnsetLabel removes our app labels from an unstructured object
func UnsetLabel(target *unstructured.Unstructured, key string) {
	if labels := target.GetLabels(); labels != nil {
		if _, ok := labels[key]; ok {
			delete(labels, key)
			if len(labels) == 0 {
				unstructured.RemoveNestedField(target.Object, "metadata", "labels")
			} else {
				target.SetLabels(labels)
			}
		}
	}
}

// SetLabel sets our app labels against an unstructured object
func SetLabel(target *unstructured.Unstructured, key, val string) error {
	labels := target.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = val
	target.SetLabels(labels)

	gvk := schema.FromAPIVersionAndKind(target.GetAPIVersion(), target.GetKind())
	// special case for deployment and job types: make sure that derived replicaset, and pod has
	// the application label
	switch gvk.Group {
	case "apps", "extensions":
		switch gvk.Kind {
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
			// spec.selector.matchLabels to match spec.template.metadata.labels. This means Argo CD
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
					// our Argo CD labels to get defaulted by kubernetes, so we explicitly set the labels
					// for them (minus the Argo CD labels).
					delete(templateLabels, key)
					err = unstructured.SetNestedMap(target.UnstructuredContent(), templateLabels, "spec", "selector", "matchLabels")
					if err != nil {
						return err
					}
				}
			}
		}
	case "batch":
		switch gvk.Kind {
		case JobKind:
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
	resList, err = disco.ServerPreferredResources()
	if err != nil {
		if len(resList) == 0 {
			return nil, errors.WithStack(err)
		}
		// It's possible for ServerResources to return error as well as a resource list
		log.Warnf("Resource discovery partially successful. Encountered error: %v", err)
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

func FlushServerResourcesCache() {
	apiResourceCache.Flush()
}

func ToGroupVersionResource(groupVersion string, apiResource *metav1.APIResource) schema.GroupVersionResource {
	gvk := schema.FromAPIVersionAndKind(groupVersion, apiResource.Kind)
	gv := gvk.GroupVersion()
	return gv.WithResource(apiResource.Name)
}

func ToResourceInterface(dynamicIf dynamic.Interface, apiResource *metav1.APIResource, resource schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if apiResource.Namespaced {
		return dynamicIf.Resource(resource).Namespace(namespace)
	}
	return dynamicIf.Resource(resource)
}

func IsCRDGroupVersionKind(gvk schema.GroupVersionKind) bool {
	return gvk.Kind == CustomResourceDefinitionKind && gvk.Group == "apiextensions.k8s.io"
}

func IsCRD(obj *unstructured.Unstructured) bool {
	return IsCRDGroupVersionKind(obj.GroupVersionKind())
}

// temporal solution for https://github.com/argoproj/argo-cd/issues/650.
func isExcludedResourceGroup(resource metav1.APIResource) bool {
	return resource.Group == "servicecatalog.k8s.io"
}

type apiResourceInterface struct {
	groupVersion string
	apiResource  metav1.APIResource
	resourceIf   dynamic.ResourceInterface
}

type filterFunc func(groupVersion string, apiResource *metav1.APIResource) bool

func filterAPIResources(config *rest.Config, filter filterFunc, namespace string) ([]apiResourceInterface, error) {
	dynamicIf, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	serverResources, err := GetCachedServerResources(config.Host, disco)
	if err != nil {
		return nil, err
	}
	apiResIfs := make([]apiResourceInterface, 0)
	for _, apiResourcesList := range serverResources {
		if gv, err := schema.ParseGroupVersion(apiResourcesList.GroupVersion); err == nil && (gv.Group == "extensions" || gv.Group == "events.k8s.io") {
			continue
		}
		for _, apiResource := range apiResourcesList.APIResources {
			if apiResource.Group == "" && apiResource.Kind == "Event" {
				continue
			}
			if filter(apiResourcesList.GroupVersion, &apiResource) {
				resource := ToGroupVersionResource(apiResourcesList.GroupVersion, &apiResource)
				resourceIf := ToResourceInterface(dynamicIf, &apiResource, resource, namespace)
				apiResIf := apiResourceInterface{
					groupVersion: apiResourcesList.GroupVersion,
					apiResource:  apiResource,
					resourceIf:   resourceIf,
				}
				apiResIfs = append(apiResIfs, apiResIf)
			}
		}
	}
	return apiResIfs, nil
}

// isSupportedVerb returns whether or not a APIResource supports a specific verb
func isSupportedVerb(apiResource *metav1.APIResource, verb string) bool {
	for _, v := range apiResource.Verbs {
		if v == verb {
			return true
		}
	}
	return false
}

func copyEventsChannel(ctx context.Context, src <-chan watch.Event, dst chan watch.Event) {
	stopped := false
	done := make(chan bool)
	go func() {
		for event := range src {
			if stopped {
				break
			}
			dst <- event
		}
		done <- true
	}()
	select {
	case <-done:
	case <-ctx.Done():
		stopped = true
	}
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
	return nil, apierr.NewNotFound(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, "")
}

// deleteFile is best effort deletion of a file
func deleteFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}
	_ = os.Remove(path)
}

// cleanKubectlOutput makes the error output of kubectl a little better to read
func cleanKubectlOutput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Replace(s, ": error validating \"STDIN\"", "", -1)
	s = strings.Replace(s, ": unable to recognize \"STDIN\"", "", -1)
	s = strings.Replace(s, ": error when creating \"STDIN\"", "", -1)
	s = strings.Replace(s, "; if you choose to ignore these errors, turn validation off with --validate=false", "", -1)
	s = strings.Replace(s, "error: error", "error", -1)
	return s
}

// WriteKubeConfig takes a rest.Config and writes it as a kubeconfig at the specified path
func WriteKubeConfig(restConfig *rest.Config, namespace, filename string) error {
	kubeConfig := NewKubeConfig(restConfig, namespace)
	return clientcmd.WriteToFile(*kubeConfig, filename)
}

// NewKubeConfig converts a clientcmdapi.Config (kubeconfig) from a rest.Config
func NewKubeConfig(restConfig *rest.Config, namespace string) *clientcmdapi.Config {
	return &clientcmdapi.Config{
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
				Server:                   restConfig.Host,
				InsecureSkipTLSVerify:    restConfig.TLSClientConfig.Insecure,
				CertificateAuthority:     restConfig.TLSClientConfig.CAFile,
				CertificateAuthorityData: restConfig.TLSClientConfig.CAData,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			restConfig.Host: newAuthInfo(restConfig),
		},
	}
}

// newAuthInfo returns an AuthInfo from a rest config, detecting if the rest.Config is an
// in-cluster config and automatically setting the token path appropriately.
func newAuthInfo(restConfig *rest.Config) *clientcmdapi.AuthInfo {
	authInfo := clientcmdapi.AuthInfo{}
	haveCredentials := false
	if restConfig.TLSClientConfig.CertFile != "" {
		authInfo.ClientCertificate = restConfig.TLSClientConfig.CertFile
		haveCredentials = true
	}
	if len(restConfig.TLSClientConfig.CertData) > 0 {
		authInfo.ClientCertificateData = restConfig.TLSClientConfig.CertData
		haveCredentials = true
	}
	if restConfig.TLSClientConfig.KeyFile != "" {
		authInfo.ClientKey = restConfig.TLSClientConfig.KeyFile
		haveCredentials = true
	}
	if len(restConfig.TLSClientConfig.KeyData) > 0 {
		authInfo.ClientKeyData = restConfig.TLSClientConfig.KeyData
		haveCredentials = true
	}
	if restConfig.Username != "" {
		authInfo.Username = restConfig.Username
		haveCredentials = true
	}
	if restConfig.Password != "" {
		authInfo.Password = restConfig.Password
		haveCredentials = true
	}
	if restConfig.BearerToken != "" {
		authInfo.Token = restConfig.BearerToken
		haveCredentials = true
	}
	if restConfig.ExecProvider != nil {
		authInfo.Exec = restConfig.ExecProvider
		haveCredentials = true
	}
	if restConfig.ExecProvider == nil && !haveCredentials {
		// If no credentials were set (or there was no exec provider), we assume in-cluster config.
		// In-cluster configs from the go-client will no longer set bearer tokens, so we set the
		// well known token path. See issue #774
		authInfo.TokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	}
	return &authInfo
}

var diffSeparator = regexp.MustCompile(`\n---`)

// SplitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If any errors occurred, returns the first one
func SplitYAML(out string) ([]*unstructured.Unstructured, error) {
	parts := diffSeparator.Split(out, -1)
	var objs []*unstructured.Unstructured
	var firstErr error
	for _, part := range parts {
		var objMap map[string]interface{}
		err := yaml.Unmarshal([]byte(part), &objMap)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("Failed to unmarshal manifest: %v", err)
			}
			continue
		}
		if len(objMap) == 0 {
			// handles case where theres no content between `---`
			continue
		}
		var obj unstructured.Unstructured
		err = yaml.Unmarshal([]byte(part), &obj)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("Failed to unmarshal manifest: %v", err)
			}
			continue
		}
		remObj, err := Remarshal(&obj)
		if err != nil {
			log.Warnf("Failed to remarshal oject: %v", err)
		} else {
			obj = *remObj
		}
		objs = append(objs, &obj)
	}
	return objs, firstErr
}

// Remarshal checks resource kind and version and re-marshal using corresponding struct custom marshaller.
// This ensures that expected resource state is formatter same as actual resource state in kubernetes
// and allows to find differences between actual and target states more accurately.
func Remarshal(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	item, err := scheme.Scheme.New(obj.GroupVersionKind())
	if err != nil {
		return nil, err
	}
	// This will drop any omitempty fields, perform resource conversion etc...
	unmarshalledObj := reflect.New(reflect.TypeOf(item).Elem()).Interface()
	err = json.Unmarshal(data, &unmarshalledObj)
	if err != nil {
		return nil, err
	}
	unstrBody, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unmarshalledObj)
	if err != nil {
		return nil, err
	}
	// remove all default values specified by custom formatter (e.g. creationTimestamp)
	unstrBody = jsonutil.RemoveMapFields(obj.Object, unstrBody)
	return &unstructured.Unstructured{Object: unstrBody}, nil
}
