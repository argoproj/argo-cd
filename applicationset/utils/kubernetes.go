package utils

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/argoproj/argo-cd/v3/common"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var ErrDisallowedSecretAccess = fmt.Errorf("secret must have label %q=%q", common.LabelKeySecretType, common.LabelValueSecretTypeSCMCreds)

// GetSecretRef gets the value of the key for the specified Secret resource.
func GetSecretRef(ctx context.Context, k8sClient client.Client, ref *argoprojiov1alpha1.SecretRef, namespace string, tokenRefStrictMode bool) (string, error) {
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

	if tokenRefStrictMode && secret.GetLabels()[common.LabelKeySecretType] != common.LabelValueSecretTypeSCMCreds {
		return "", fmt.Errorf("secret %s/%s is not a valid SCM creds secret: %w", namespace, ref.SecretName, ErrDisallowedSecretAccess)
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

// GetTLSClientCertificate loads a client certificate and key from a single Secret snapshot.
func GetTLSClientCertificate(ctx context.Context, k8sClient client.Client, ref *argoprojiov1alpha1.TLSClientCertSecretReference, namespace string, strictMode bool) (*tls.Certificate, error) {
	if ref == nil {
		return nil, nil
	}
	if ref.Name == "" {
		return nil, errors.New("TLS client certificate Secret name must not be empty")
	}
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, secret); err != nil {
		return nil, fmt.Errorf("error fetching TLS client certificate Secret %s/%s: %w", namespace, ref.Name, err)
	}
	if strictMode && secret.GetLabels()[common.LabelKeySecretType] != common.LabelValueSecretTypeSCMCreds {
		return nil, fmt.Errorf("secret %s/%s is not a valid SCM creds Secret: %w", namespace, ref.Name, ErrDisallowedSecretAccess)
	}
	for _, key := range []string{corev1.TLSCertKey, corev1.TLSPrivateKeyKey} {
		if len(secret.Data[key]) == 0 {
			return nil, fmt.Errorf("key %q in TLS client certificate Secret %s/%s is missing or empty", key, namespace, ref.Name)
		}
	}
	cert, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return nil, fmt.Errorf("invalid TLS client certificate in Secret %s/%s: %w", namespace, ref.Name, err)
	}
	return &cert, nil
}
