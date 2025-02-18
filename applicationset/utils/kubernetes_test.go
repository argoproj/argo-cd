package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestGetSecretRef(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: "test"},
		Data: map[string][]byte{
			"my-token": []byte("secret"),
		},
	}
	client := fake.NewClientBuilder().WithObjects(secret).Build()
	ctx := context.Background()

	cases := []struct {
		name, namespace, token string
		ref                    *argoprojiov1alpha1.SecretRef
		hasError               bool
	}{
		{
			name:      "valid ref",
			ref:       &argoprojiov1alpha1.SecretRef{SecretName: "test-secret", Key: "my-token"},
			namespace: "test",
			token:     "secret",
			hasError:  false,
		},
		{
			name:      "nil ref",
			ref:       nil,
			namespace: "test",
			token:     "",
			hasError:  false,
		},
		{
			name:      "wrong name",
			ref:       &argoprojiov1alpha1.SecretRef{SecretName: "other", Key: "my-token"},
			namespace: "test",
			token:     "",
			hasError:  true,
		},
		{
			name:      "wrong key",
			ref:       &argoprojiov1alpha1.SecretRef{SecretName: "test-secret", Key: "other-token"},
			namespace: "test",
			token:     "",
			hasError:  true,
		},
		{
			name:      "wrong namespace",
			ref:       &argoprojiov1alpha1.SecretRef{SecretName: "test-secret", Key: "my-token"},
			namespace: "other",
			token:     "",
			hasError:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			token, err := GetSecretRef(ctx, client, c.ref, c.namespace)
			if c.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, c.token, token)
		})
	}
}

func TestGetConfigMapData(t *testing.T) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-configmap", Namespace: "test"},
		Data: map[string]string{
			"my-data": "configmap-data",
		},
	}
	client := fake.NewClientBuilder().WithObjects(configMap).Build()
	ctx := context.Background()

	cases := []struct {
		name, namespace, data string
		ref                   *argoprojiov1alpha1.ConfigMapKeyRef
		hasError              bool
	}{
		{
			name:      "valid ref",
			ref:       &argoprojiov1alpha1.ConfigMapKeyRef{ConfigMapName: "test-configmap", Key: "my-data"},
			namespace: "test",
			data:      "configmap-data",
			hasError:  false,
		},
		{
			name:      "nil ref",
			ref:       nil,
			namespace: "test",
			data:      "",
			hasError:  false,
		},
		{
			name:      "wrong name",
			ref:       &argoprojiov1alpha1.ConfigMapKeyRef{ConfigMapName: "other", Key: "my-data"},
			namespace: "test",
			data:      "",
			hasError:  true,
		},
		{
			name:      "wrong key",
			ref:       &argoprojiov1alpha1.ConfigMapKeyRef{ConfigMapName: "test-configmap", Key: "other-data"},
			namespace: "test",
			data:      "",
			hasError:  true,
		},
		{
			name:      "wrong namespace",
			ref:       &argoprojiov1alpha1.ConfigMapKeyRef{ConfigMapName: "test-configmap", Key: "my-data"},
			namespace: "other",
			data:      "",
			hasError:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := GetConfigMapData(ctx, client, c.ref, c.namespace)
			if c.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if !c.hasError {
				assert.Equal(t, c.data, string(data))
			}
		})
	}
}
