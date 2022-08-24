package kube

// Kubernetes client related code

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/metrics"

	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesClient struct {
	Clientset             kubernetes.Interface
	ApplicationsClientset versioned.Interface
	Context               context.Context
	Namespace             string
}

func NewKubernetesClient(ctx context.Context, client kubernetes.Interface, applicationsClientset versioned.Interface, namespace string) *KubernetesClient {
	kc := &KubernetesClient{}
	kc.Context = ctx
	kc.Clientset = client
	kc.ApplicationsClientset = applicationsClientset
	kc.Namespace = namespace
	return kc
}

// NewKubernetesClient creates a new Kubernetes client object from given
// configuration file. If configuration file is the empty string, in-cluster
// client will be created.
func NewKubernetesClientFromConfig(ctx context.Context, namespace string, kubeconfig string) (*KubernetesClient, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	loadingRules.ExplicitPath = kubeconfig
	overrides := clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	if namespace == "" {
		namespace, _, err = clientConfig.Namespace()
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	applicationsClientset, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return NewKubernetesClient(ctx, clientset, applicationsClientset, namespace), nil
}

// GetSecretData returns the raw data from named K8s secret in given namespace
func (client *KubernetesClient) GetSecretData(namespace string, secretName string) (map[string][]byte, error) {
	secret, err := client.Clientset.CoreV1().Secrets(namespace).Get(client.Context, secretName, metav1.GetOptions{})
	metrics.Clients().IncreaseK8sClientRequest(1)
	if err != nil {
		metrics.Clients().IncreaseK8sClientRequest(1)
		return nil, err
	}
	return secret.Data, nil
}

// GetSecretField returns the value of a field from named K8s secret in given namespace
func (client *KubernetesClient) GetSecretField(namespace string, secretName string, field string) (string, error) {
	secret, err := client.GetSecretData(namespace, secretName)
	metrics.Clients().IncreaseK8sClientRequest(1)
	if err != nil {
		metrics.Clients().IncreaseK8sClientRequest(1)
		return "", err
	}
	if data, ok := secret[field]; !ok {
		return "", fmt.Errorf("secret '%s/%s' does not have a field '%s'", namespace, secretName, field)
	} else {
		return string(data), nil
	}
}

// CreateApplicationevent creates a kubernetes event with a custom reason and message for an application.
func (client *KubernetesClient) CreateApplicationEvent(app *appv1alpha1.Application, reason string, message string, annotations map[string]string) (*v1.Event, error) {
	t := metav1.Time{Time: time.Now()}

	event := v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%v.%x", app.ObjectMeta.Name, t.UnixNano()),
			Namespace:   client.Namespace,
			Annotations: annotations,
		},
		Source: v1.EventSource{
			Component: "ArgocdImageUpdater",
		},
		InvolvedObject: v1.ObjectReference{
			Kind:            app.Kind,
			APIVersion:      app.APIVersion,
			Name:            app.ObjectMeta.Name,
			Namespace:       app.ObjectMeta.Namespace,
			ResourceVersion: app.ObjectMeta.ResourceVersion,
			UID:             app.ObjectMeta.UID,
		},
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Message:        message,
		Type:           v1.EventTypeNormal,
		Reason:         reason,
	}

	result, err := client.Clientset.CoreV1().Events(client.Namespace).Create(client.Context, &event, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return result, nil
}
