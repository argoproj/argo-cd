package k8s

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	settingsResyncDuration = 3 * time.Minute
)

func NewSecretInformer(clientset kubernetes.Interface, namespace string, secretName string) cache.SharedIndexInformer {
	return informersv1.NewFilteredSecretInformer(clientset, namespace, settingsResyncDuration, cache.Indexers{}, func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=" + secretName
	})
}

func NewConfigMapInformer(clientset kubernetes.Interface, namespace string, configMapName string) cache.SharedIndexInformer {
	return informersv1.NewFilteredConfigMapInformer(clientset, namespace, settingsResyncDuration, cache.Indexers{}, func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=" + configMapName
	})
}
