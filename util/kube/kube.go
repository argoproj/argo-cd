// Package kube provides helper utilities common for kubernetes
package kube

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
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

	"github.com/argoproj/argo-cd/common"
)

const (
	listVerb  = "list"
	watchVerb = "watch"
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
	APIServiceKind               = "APIService"
)

type ResourceKey struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
}

func (k *ResourceKey) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", k.Group, k.Kind, k.Namespace, k.Name)
}

func (k ResourceKey) GroupKind() schema.GroupKind {
	return schema.GroupKind{Group: k.Group, Kind: k.Kind}
}

func NewResourceKey(group string, kind string, namespace string, name string) ResourceKey {
	return ResourceKey{Group: group, Kind: kind, Namespace: namespace, Name: name}
}

func GetResourceKey(obj *unstructured.Unstructured) ResourceKey {
	gvk := obj.GroupVersionKind()
	return NewResourceKey(gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName())
}

func GetObjectRef(obj *unstructured.Unstructured) v1.ObjectReference {
	return v1.ObjectReference{
		UID:        obj.GetUID(),
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
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

// GetAppInstanceLabel returns the application instance name from labels
func GetAppInstanceLabel(un *unstructured.Unstructured, key string) string {
	if labels := un.GetLabels(); labels != nil {
		return labels[key]
	}
	return ""
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

// SetAppInstanceLabel the recommended app.kubernetes.io/instance label against an unstructured object
// Uses the legacy labeling if environment variable is set
func SetAppInstanceLabel(target *unstructured.Unstructured, key, val string) error {
	labels := target.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = val
	target.SetLabels(labels)
	if key != common.LabelKeyLegacyApplicationName {
		// we no longer label the pod template sub resources in v0.11
		return nil
	}

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
		objs = append(objs, &obj)
	}
	return objs, firstErr
}

// WatchWithRetry returns channel of watch events or errors of failed to call watch API.
func WatchWithRetry(ctx context.Context, getWatch func() (watch.Interface, error)) chan struct {
	*watch.Event
	Error error
} {
	ch := make(chan struct {
		*watch.Event
		Error error
	})
	execute := func() (bool, error) {
		w, err := getWatch()
		if err != nil {
			return false, err
		}
		defer w.Stop()

		for {
			select {
			case event, ok := <-w.ResultChan():
				if ok {
					ch <- struct {
						*watch.Event
						Error error
					}{Event: &event, Error: nil}
				} else {
					return true, nil
				}
			case <-ctx.Done():
				return false, nil
			}
		}
	}
	go func() {
		defer close(ch)
		for {
			retry, err := execute()
			if err != nil {
				ch <- struct {
					*watch.Event
					Error error
				}{Error: err}
			}
			if !retry {
				return
			}
			time.Sleep(time.Second)
		}
	}()
	return ch
}

func GetDeploymentReplicas(u *unstructured.Unstructured) *int64 {
	val, found, err := unstructured.NestedInt64(u.Object, "spec", "replicas")
	if !found || err != nil {
		return nil
	}
	return &val
}
