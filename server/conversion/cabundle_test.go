package conversion

import (
	"crypto/tls"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	tlsutil "github.com/argoproj/argo-cd/v3/util/tls"
)

func generateTestCert(t *testing.T) *tls.Certificate {
	t.Helper()
	cert, err := tlsutil.GenerateX509KeyPair(tlsutil.CertOptions{
		Hosts:        []string{"argocd-conversion-webhook", "argocd-conversion-webhook.argocd.svc"},
		Organization: "Argo CD Test",
		IsCA:         true,
	})
	require.NoError(t, err)
	return cert
}

func certPEM(t *testing.T, cert *tls.Certificate) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate[len(cert.Certificate)-1],
	})
}

func newAppCRD(caBundle []byte) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: application.ApplicationFullName},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Conversion: &apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.WebhookConverter,
				Webhook: &apiextensionsv1.WebhookConversion{
					ClientConfig: &apiextensionsv1.WebhookClientConfig{CABundle: caBundle},
				},
			},
		},
	}
}

func getCABundle(t *testing.T, client *apiextensionsfake.Clientset) []byte {
	t.Helper()
	crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(t.Context(), application.ApplicationFullName, metav1.GetOptions{})
	require.NoError(t, err)
	return crd.Spec.Conversion.Webhook.ClientConfig.CABundle
}

func TestInjectCABundle_EmptyBundleIsInjected(t *testing.T) {
	cert := generateTestCert(t)
	client := apiextensionsfake.NewSimpleClientset(newAppCRD(nil))

	require.NoError(t, injectCABundle(t.Context(), client, cert))

	assert.Equal(t, certPEM(t, cert), getCABundle(t, client))
}

func TestInjectCABundle_MatchingBundleLeftUnchanged(t *testing.T) {
	cert := generateTestCert(t)
	client := apiextensionsfake.NewSimpleClientset(newAppCRD(certPEM(t, cert)))
	client.ClearActions()

	require.NoError(t, injectCABundle(t.Context(), client, cert))

	for _, action := range client.Actions() {
		assert.NotEqual(t, "patch", action.GetVerb(), "matching CA bundle must not be re-patched")
	}
}

func TestInjectCABundle_StaleBundleIsReplaced(t *testing.T) {
	// Simulates a certificate rotation: the CRD still carries the previous
	// certificate's CA, which cannot verify the cert the new pod serves. The
	// previous CA must survive the merge so pods still serving the previous
	// certificate keep working until they restart.
	previous := generateTestCert(t)
	current := generateTestCert(t)
	client := apiextensionsfake.NewSimpleClientset(newAppCRD(certPEM(t, previous)))

	require.NoError(t, injectCABundle(t.Context(), client, current))

	bundle := getCABundle(t, client)
	assert.Equal(t, append(certPEM(t, previous), certPEM(t, current)...), bundle)
	assert.True(t, caBundleVerifiesCert(bundle, previous))
	assert.True(t, caBundleVerifiesCert(bundle, current))
}

func TestInjectCABundle_ExpiredEntriesAreDropped(t *testing.T) {
	expired, err := tlsutil.GenerateX509KeyPair(tlsutil.CertOptions{
		Hosts:        []string{"argocd-conversion-webhook"},
		Organization: "Argo CD Test",
		IsCA:         true,
		ValidFrom:    time.Now().Add(-2 * time.Hour),
		ValidFor:     time.Hour,
	})
	require.NoError(t, err)
	current := generateTestCert(t)
	client := apiextensionsfake.NewSimpleClientset(newAppCRD(certPEM(t, expired)))

	require.NoError(t, injectCABundle(t.Context(), client, current))

	assert.Equal(t, certPEM(t, current), getCABundle(t, client))
}

func TestInjectCABundle_GarbageBundleIsReplaced(t *testing.T) {
	current := generateTestCert(t)
	client := apiextensionsfake.NewSimpleClientset(newAppCRD([]byte("not pem")))

	require.NoError(t, injectCABundle(t.Context(), client, current))

	assert.Equal(t, certPEM(t, current), getCABundle(t, client))
}

func TestInjectCABundle_NoWebhookConversionIsNoop(t *testing.T) {
	cert := generateTestCert(t)
	crd := newAppCRD(nil)
	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.NoneConverter}
	client := apiextensionsfake.NewSimpleClientset(crd)
	client.ClearActions()

	require.NoError(t, injectCABundle(t.Context(), client, cert))

	for _, action := range client.Actions() {
		assert.NotEqual(t, "patch", action.GetVerb())
	}
}

func TestInjectCABundle_NilCertIsNoop(t *testing.T) {
	client := apiextensionsfake.NewSimpleClientset(newAppCRD(nil))
	require.NoError(t, injectCABundle(t.Context(), client, nil))
	assert.Empty(t, getCABundle(t, client))
}

func TestCABundleVerifiesCert(t *testing.T) {
	cert := generateTestCert(t)
	other := generateTestCert(t)

	assert.True(t, caBundleVerifiesCert(certPEM(t, cert), cert))
	assert.False(t, caBundleVerifiesCert(certPEM(t, other), cert))
	assert.False(t, caBundleVerifiesCert([]byte("not pem"), cert))
}
