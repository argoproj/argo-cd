// Package kube provides helper utilities common for kubernetes
package kube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

const (
	listVerb  = "list"
	watchVerb = "watch"
)

const (
	SecretKind                   = "Secret"
	ServiceKind                  = "Service"
	ServiceAccountKind           = "ServiceAccount"
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
	NamespaceKind                = "Namespace"
	HorizontalPodAutoscalerKind  = "HorizontalPodAutoscaler"
)

type ResourceInfoProvider interface {
	IsNamespaced(gk schema.GroupKind) (bool, error)
}

func IsNamespacedOrUnknown(provider ResourceInfoProvider, gk schema.GroupKind) bool {
	namespaced, err := provider.IsNamespaced(gk)
	return namespaced || err != nil
}

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

func GetObjectRef(obj *unstructured.Unstructured) corev1.ObjectReference {
	return corev1.ObjectReference{
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
		return fmt.Errorf("REST config invalid: %w", err)
	}
	_, err = kubeclientset.ServerVersion()
	if err != nil {
		return fmt.Errorf("REST config invalid: %w", err)
	}
	return nil
}

// ToUnstructured converts a concrete K8s API type to a un unstructured object
func ToUnstructured(obj any) (*unstructured.Unstructured, error) {
	uObj, err := runtime.NewTestUnstructuredConverter(equality.Semantic).ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: uObj}, nil
}

// MustToUnstructured converts a concrete K8s API type to a un unstructured object and panics if not successful
func MustToUnstructured(obj any) *unstructured.Unstructured {
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

// ServerResourceForGroupVersionKind looks up and returns the API resource from
// the server for a given GVK scheme. If verb is set to the non-empty string,
// it will return the API resource which supports the verb. There are some edge
// cases, where the same GVK is represented by more than one API.
//
// See: https://github.com/ksonnet/ksonnet/blob/master/utils/client.go
func ServerResourceForGroupVersionKind(disco discovery.DiscoveryInterface, gvk schema.GroupVersionKind, verb string) (*metav1.APIResource, error) {
	// default is to return a not found for the requested resource
	retErr := apierrors.NewNotFound(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, "")
	resources, err := disco.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return nil, err
	}
	for _, r := range resources.APIResources {
		if r.Kind == gvk.Kind {
			if isSupportedVerb(&r, verb) {
				return &r, nil
			}
			// We have a match, but the API does not support the action
			// that was requested. Memorize this.
			retErr = apierrors.NewMethodNotSupported(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, verb)
		}
	}
	return nil, retErr
}

var (
	kubectlErrOutRegexp = regexp.MustCompile(`^(error: )?(error validating|error when creating|error when creating) "\S+": `)

	// See ApplyOpts::Run()
	// cmdutil.AddSourceToErr(fmt.Sprintf("applying patch:\n%s\nto:\n%v\nfor:", patchBytes, info), info.Source, err)
	kubectlApplyPatchErrOutRegexp = regexp.MustCompile(`(?s)^error when applying patch:.*\nfor: "\S+": `)

	kubectlErrOutMapRegexp = regexp.MustCompile(`map\[.*\]`)
)

// cleanKubectlOutput makes the error output of kubectl a little better to read
func cleanKubectlOutput(s string) string {
	s = strings.TrimSpace(s)
	s = kubectlErrOutRegexp.ReplaceAllString(s, "")
	s = kubectlErrOutMapRegexp.ReplaceAllString(s, "")
	s = kubectlApplyPatchErrOutRegexp.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "; if you choose to ignore these errors, turn validation off with --validate=false", "")
	return s
}

// WriteKubeConfig takes a rest.Config and writes it as a kubeconfig at the specified path
func WriteKubeConfig(restConfig *rest.Config, namespace, filename string) error {
	kubeConfig := NewKubeConfig(restConfig, namespace)
	return clientcmd.WriteToFile(*kubeConfig, filename)
}

// NewKubeConfig converts a clientcmdapi.Config (kubeconfig) from a rest.Config
func NewKubeConfig(restConfig *rest.Config, namespace string) *clientcmdapi.Config {
	var proxyUrl string
	if restConfig.Proxy != nil {
		if u, err := restConfig.Proxy(nil); err == nil {
			proxyUrl = u.String()
		}
	}
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
				TLSServerName:            restConfig.TLSClientConfig.ServerName,
				InsecureSkipTLSVerify:    restConfig.TLSClientConfig.Insecure,
				CertificateAuthority:     restConfig.TLSClientConfig.CAFile,
				CertificateAuthorityData: restConfig.TLSClientConfig.CAData,
				ProxyURL:                 proxyUrl,
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

// SplitYAML splits a YAML file into unstructured objects. Returns list of all unstructured objects
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
func SplitYAML(yamlData []byte) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	ymls, err := SplitYAMLToString(yamlData)
	if err != nil {
		return nil, err
	}
	for _, yml := range ymls {
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(yml), u); err != nil {
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		objs = append(objs, u)
	}
	return objs, nil
}

// SplitYAMLToString splits a YAML file into strings. Returns list of yamls
// found in the yaml. If an error occurs, returns objects that have been parsed so far too.
func SplitYAMLToString(yamlData []byte) ([]string, error) {
	// Similar way to what kubectl does
	// https://github.com/kubernetes/cli-runtime/blob/master/pkg/resource/visitor.go#L573-L600
	// Ideally k8s.io/cli-runtime/pkg/resource.Builder should be used instead of this method.
	// E.g. Builder does list unpacking and flattening and this code does not.
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 4096)
	var objs []string
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		objs = append(objs, string(ext.Raw))
	}
	return objs, nil
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

func GetResourceImages(u *unstructured.Unstructured) []string {
	var containers []any
	var found bool
	var err error
	var images []string

	containerPaths := [][]string{
		// Resources without template, like pods
		{"spec", "containers"},
		// Resources with template, like deployments
		{"spec", "template", "spec", "containers"},
		// Cronjobs
		{"spec", "jobTemplate", "spec", "template", "spec", "containers"},
	}

	for _, path := range containerPaths {
		containers, found, err = unstructured.NestedSlice(u.Object, path...)
		if found && err == nil {
			break
		}
	}

	if !found || err != nil {
		return nil
	}

	for _, container := range containers {
		containerMap, ok := container.(map[string]any)
		if !ok {
			continue
		}

		image, found, err := unstructured.NestedString(containerMap, "image")
		if !found || err != nil {
			continue
		}

		images = append(images, image)
	}

	return images
}

// RetryUntilSucceed keep retrying given action with specified interval until action succeed or specified context is done.
func RetryUntilSucceed(ctx context.Context, interval time.Duration, desc string, log logr.Logger, action func() error) {
	pollErr := wait.PollUntilContextCancel(ctx, interval, true, func(_ context.Context) (bool /*done*/, error) {
		log.V(1).Info("Start " + desc)
		err := action()
		if err == nil {
			log.V(1).Info("Completed " + desc)
			return true, nil
		}
		log.V(1).Info(fmt.Sprintf("Failed to %s: %+v, retrying in %v", desc, err, interval))
		return false, nil
	})
	if pollErr != nil {
		// The only error that can happen here is wait.ErrWaitTimeout if ctx is done.
		log.V(1).Info("Stop retrying " + desc)
	}
}
