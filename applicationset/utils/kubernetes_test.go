package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/common"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestGetSecretRef(t *testing.T) {
	t.Parallel()
	secret := &corev1.Secret{
		Name: "test-secret", Namespace: "test",
		Data: map[string][]byte{
			"my-token": []byte("secret"),
		},
	}
	client := fake.NewClientBuilder().WithObjects(secret).Build()
	ctx := t.Context()

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
			t.Parallel()
			token, err := GetSecretRef(ctx, client, c.ref, c.namespace, false)
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
	t.Parallel()
	configMap := &corev1.ConfigMap{
		Name: "test-configmap", Namespace: "test",
		Data: map[string]string{
			"my-data": "configmap-data",
		},
	}
	client := fake.NewClientBuilder().WithObjects(configMap).Build()
	ctx := t.Context()

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
			t.Parallel()
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

// TestGetTLSClientCertificate checks Secret lookup, namespace and label restrictions, and certificate/key validation without disclosing private keys.
func TestGetTLSClientCertificate(t *testing.T) {
	t.Parallel()
	certPEM, err := os.ReadFile("../../test/fixture/certs/argocd-test-client.crt")
	require.NoError(t, err)
	keyPEM, err := os.ReadFile("../../test/fixture/certs/argocd-test-client.key")
	require.NoError(t, err)
	otherKey, err := os.ReadFile("../../test/fixture/certs/argocd-test-server.key")
	require.NoError(t, err)
	for _, tc := range []struct {
		name            string
		ref             *argoprojiov1alpha1.TLSClientCertSecretReference
		namespace       string
		strict, labeled bool
		cert, key       []byte
		wantErr         string
	}{
		{name: "not configured"},
		{name: "valid", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "test", cert: certPEM, key: keyPEM},
		{name: "strict labeled", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "test", strict: true, labeled: true, cert: certPEM, key: keyPEM},
		{name: "strict unlabeled", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "test", strict: true, cert: certPEM, key: keyPEM, wantErr: "not a valid SCM creds Secret"},
		{name: "empty name", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{}, namespace: "test", wantErr: "name must not be empty"},
		{name: "missing Secret", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "missing"}, namespace: "test", wantErr: "not found"},
		{name: "other namespace", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "other", wantErr: "not found"},
		{name: "missing certificate", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "test", key: keyPEM, wantErr: `key "tls.crt"`},
		{name: "missing key", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "test", cert: certPEM, wantErr: `key "tls.key"`},
		{name: "empty certificate", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "test", cert: []byte{}, key: keyPEM, wantErr: `key "tls.crt"`},
		{name: "malformed certificate", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "test", cert: []byte("invalid-client-certificate"), key: keyPEM, wantErr: "invalid TLS client certificate"},
		{name: "malformed key", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "test", cert: certPEM, key: []byte("invalid-private-key"), wantErr: "invalid TLS client certificate"},
		{name: "mismatched key", ref: &argoprojiov1alpha1.TLSClientCertSecretReference{Name: "client"}, namespace: "test", cert: certPEM, key: otherKey, wantErr: "private key does not match public key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			secret := &corev1.Secret{Name: "client", Namespace: "test", Type: corev1.SecretTypeTLS, Data: map[string][]byte{corev1.TLSCertKey: tc.cert, corev1.TLSPrivateKeyKey: tc.key}}
			if tc.labeled {
				secret.Labels = map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeSCMCreds}
			}
			k8sClient := fake.NewClientBuilder().WithObjects(secret).Build()
			cert, err := GetTLSClientCertificate(t.Context(), k8sClient, tc.ref, tc.namespace, tc.strict)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				assert.Nil(t, cert)
				assert.NotContains(t, err.Error(), "invalid-private-key")
				assert.NotContains(t, err.Error(), string(keyPEM))
				if tc.strict && !tc.labeled {
					require.ErrorIs(t, err, ErrDisallowedSecretAccess)
				}
				return
			}
			require.NoError(t, err)
			if tc.ref == nil {
				assert.Nil(t, cert)
			} else {
				require.NotNil(t, cert)
				assert.NotEmpty(t, cert.Certificate)
				assert.NotNil(t, cert.PrivateKey)
			}
		})
	}
}
