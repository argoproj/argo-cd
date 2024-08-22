package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// getSecretRef gets the value of the key for the specified Secret resource.
func GetSecretRef(ctx context.Context, k8sClient client.Client, ref *argoprojiov1alpha1.SecretRef, namespace string) (string, error) {
	if ref == nil {
		return "", nil
	}

	secret := &corev1.Secret{}
	err := k8sClient.Get(
		ctx,
		client.ObjectKey{
			Name:      ref.SecretName,
			Namespace: namespace,
		},
		secret)
	if err != nil {
		return "", fmt.Errorf("error fetching secret %s/%s: %w", namespace, ref.SecretName, err)
	}
	tokenBytes, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %q in secret %s/%s not found", ref.Key, namespace, ref.SecretName)
	}
	return string(tokenBytes), nil
}

func GetConfigMapData(ctx context.Context, k8sClient client.Client, ref *argoprojiov1alpha1.ConfigMapKeyRef, namespace string) ([]byte, error) {
	if ref == nil {
		return nil, nil
	}

	configMap := &corev1.ConfigMap{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: ref.ConfigMapName, Namespace: namespace}, configMap)
	if err != nil {
		return nil, err
	}

	data, ok := configMap.Data[ref.Key]
	if !ok {
		return nil, fmt.Errorf("key %s not found in ConfigMap %s", ref.Key, configMap.Name)
	}

	return []byte(data), nil
}
