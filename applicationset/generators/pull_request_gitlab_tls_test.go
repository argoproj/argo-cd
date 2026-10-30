package generators

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/common"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestPullRequestGitLabMutualTLSGenerateParams(t *testing.T) {
	t.Parallel()
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile("../../test/fixture/certs/" + name)
		require.NoError(t, err)
		return data
	}
	ca := read("argocd-test-ca.crt")
	certPEM, keyPEM := read("argocd-test-client.crt"), read("argocd-test-client.key")
	serverCert, err := tls.X509KeyPair(read("argocd-test-server.crt"), read("argocd-test-server.key"))
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(ca))
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "api-token", r.Header.Get("Private-Token"))
		require.NotEmpty(t, r.TLS.VerifiedChains)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`[{"iid":7,"title":"Preview","source_branch":"feature","target_branch":"main","sha":"abcd1234","author":{"username":"dev"}}]`))
		assert.NoError(t, err)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}
	server.StartTLS()
	defer server.Close()
	secret := &corev1.Secret{Name: "gitlab", Namespace: "test", Labels: map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeSCMCreds}, Data: map[string][]byte{corev1.TLSCertKey: certPEM, corev1.TLSPrivateKeyKey: keyPEM, "token": []byte("api-token")}}
	caMap := &corev1.ConfigMap{Name: "gitlab-ca", Namespace: "test", Data: map[string]string{"ca.crt": string(ca)}}
	k8sClient := fake.NewClientBuilder().WithObjects(secret, caMap).Build()
	gen := NewPullRequestGenerator(k8sClient, NewSCMConfig("", []string{server.URL}, true, false, nil, true))
	appset := &argoprojiov1alpha1.ApplicationSet{Namespace: "test"}
	cfg := &argoprojiov1alpha1.ApplicationSetGenerator{PullRequest: &argoprojiov1alpha1.PullRequestGenerator{GitLab: &argoprojiov1alpha1.PullRequestGeneratorGitLab{
		API: server.URL, Project: "42", TokenRef: &argoprojiov1alpha1.SecretRef{SecretName: "gitlab", Key: "token"}, CARef: &argoprojiov1alpha1.ConfigMapKeyRef{ConfigMapName: "gitlab-ca", Key: "ca.crt"}, TLSClientCertSecretRef: &corev1.LocalObjectReference{Name: "gitlab"},
	}}}
	params, err := gen.GenerateParams(cfg, appset, nil)
	require.NoError(t, err)
	require.Len(t, params, 1)
	assert.Equal(t, "7", params[0]["number"])
	assert.Equal(t, "abcd1234", params[0]["head_sha"])
	// Renewal is read on the next generation, rather than cached in the generator.
	secret.Data[corev1.TLSPrivateKeyKey] = []byte("invalid-rotated-private-key")
	require.NoError(t, k8sClient.Update(t.Context(), secret))
	_, err = gen.GenerateParams(cfg, appset, nil)
	require.ErrorContains(t, err, "invalid TLS client certificate")
	assert.NotContains(t, err.Error(), "invalid-rotated-private-key")
	secret.Data[corev1.TLSPrivateKeyKey] = keyPEM
	require.NoError(t, k8sClient.Update(t.Context(), secret))
	params, err = gen.GenerateParams(cfg, appset, nil)
	require.NoError(t, err)
	require.Len(t, params, 1)
}
