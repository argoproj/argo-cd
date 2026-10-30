package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/common"
)

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
		ref             *corev1.LocalObjectReference
		namespace       string
		strict, labeled bool
		cert, key       []byte
		wantErr         string
	}{
		{name: "not configured"},
		{name: "valid", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "test", cert: certPEM, key: keyPEM},
		{name: "strict labeled", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "test", strict: true, labeled: true, cert: certPEM, key: keyPEM},
		{name: "strict unlabeled", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "test", strict: true, cert: certPEM, key: keyPEM, wantErr: "not a valid SCM creds Secret"},
		{name: "empty name", ref: &corev1.LocalObjectReference{}, namespace: "test", wantErr: "name must not be empty"},
		{name: "missing Secret", ref: &corev1.LocalObjectReference{Name: "missing"}, namespace: "test", wantErr: "not found"},
		{name: "other namespace", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "other", wantErr: "not found"},
		{name: "missing certificate", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "test", key: keyPEM, wantErr: `key "tls.crt"`},
		{name: "missing key", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "test", cert: certPEM, wantErr: `key "tls.key"`},
		{name: "empty certificate", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "test", cert: []byte{}, key: keyPEM, wantErr: `key "tls.crt"`},
		{name: "malformed certificate", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "test", cert: []byte("invalid-client-certificate"), key: keyPEM, wantErr: "invalid TLS client certificate"},
		{name: "malformed key", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "test", cert: certPEM, key: []byte("invalid-private-key"), wantErr: "invalid TLS client certificate"},
		{name: "mismatched key", ref: &corev1.LocalObjectReference{Name: "client"}, namespace: "test", cert: certPEM, key: otherKey, wantErr: "private key does not match public key"},
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
