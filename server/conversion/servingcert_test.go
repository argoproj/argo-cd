package conversion

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	tlsutil "github.com/argoproj/argo-cd/v3/util/tls"
)

const testNamespace = "argocd"

var testHosts = []string{
	"localhost",
	"argocd-conversion-webhook",
	"argocd-conversion-webhook.argocd",
	"argocd-conversion-webhook.argocd.svc",
	"argocd-conversion-webhook.argocd.svc.cluster.local",
}

func tlsSecret(t *testing.T, cert *tls.Certificate) *corev1.Secret {
	t.Helper()
	certPEM, keyPEM := tlsutil.EncodeX509KeyPair(*cert)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: TLSSecretName, Namespace: testNamespace},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certPEM,
			corev1.TLSPrivateKeyKey: keyPEM,
		},
	}
}

func getTLSSecret(t *testing.T, client *kubefake.Clientset) *corev1.Secret {
	t.Helper()
	secret, err := client.CoreV1().Secrets(testNamespace).Get(t.Context(), TLSSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	return secret
}

func generateHostCert(t *testing.T, hosts []string, validFor time.Duration) *tls.Certificate {
	t.Helper()
	cert, err := tlsutil.GenerateX509KeyPair(tlsutil.CertOptions{
		Hosts:        hosts,
		Organization: "Argo CD Test",
		IsCA:         true,
		ValidFor:     validFor,
	})
	require.NoError(t, err)
	return cert
}

func TestEnsureSecretCert_CreatesSecretWhenMissing(t *testing.T) {
	client := kubefake.NewSimpleClientset()

	cert, err := ensureSecretCert(t.Context(), client, testNamespace, testHosts)
	require.NoError(t, err)
	require.NotNil(t, cert.Leaf)

	for _, host := range testHosts {
		require.NoError(t, cert.Leaf.VerifyHostname(host))
	}

	secret := getTLSSecret(t, client)
	assert.Equal(t, corev1.SecretTypeTLS, secret.Type)
	stored, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	require.NoError(t, err)
	assert.Equal(t, cert.Certificate, stored.Certificate, "served certificate must be the one persisted in the secret")
}

func TestEnsureSecretCert_ReusesStoredCert(t *testing.T) {
	stored := generateHostCert(t, testHosts, 365*24*time.Hour)
	client := kubefake.NewSimpleClientset(tlsSecret(t, stored))
	client.ClearActions()

	cert, err := ensureSecretCert(t.Context(), client, testNamespace, testHosts)
	require.NoError(t, err)

	assert.Equal(t, stored.Certificate, cert.Certificate)
	for _, action := range client.Actions() {
		assert.Contains(t, []string{"get"}, action.GetVerb(), "a valid stored certificate must not be rewritten")
	}
}

func TestEnsureSecretCert_RotatesExpiringCert(t *testing.T) {
	// Valid, but inside the 30-day renewal margin.
	expiring := generateHostCert(t, testHosts, 24*time.Hour)
	client := kubefake.NewSimpleClientset(tlsSecret(t, expiring))

	cert, err := ensureSecretCert(t.Context(), client, testNamespace, testHosts)
	require.NoError(t, err)

	assert.NotEqual(t, expiring.Certificate, cert.Certificate, "expiring certificate must be rotated")
	secret := getTLSSecret(t, client)
	stored, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	require.NoError(t, err)
	assert.Equal(t, cert.Certificate, stored.Certificate)
}

func TestEnsureSecretCert_RotatesCertMissingHost(t *testing.T) {
	// Issued for a different service DNS name, e.g. after a namespace move.
	wrongHosts := generateHostCert(t, []string{"argocd-conversion-webhook.other-ns.svc"}, 365*24*time.Hour)
	client := kubefake.NewSimpleClientset(tlsSecret(t, wrongHosts))

	cert, err := ensureSecretCert(t.Context(), client, testNamespace, testHosts)
	require.NoError(t, err)

	require.NotNil(t, cert.Leaf)
	for _, host := range testHosts {
		require.NoError(t, cert.Leaf.VerifyHostname(host))
	}
}

func TestEnsureSecretCert_RotatesGarbageSecret(t *testing.T) {
	garbage := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: TLSSecretName, Namespace: testNamespace},
		Data:       map[string][]byte{corev1.TLSCertKey: []byte("not a cert"), corev1.TLSPrivateKeyKey: []byte("not a key")},
	}
	client := kubefake.NewSimpleClientset(garbage)

	cert, err := ensureSecretCert(t.Context(), client, testNamespace, testHosts)
	require.NoError(t, err)
	require.NotNil(t, cert.Leaf)

	secret := getTLSSecret(t, client)
	_, err = tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	require.NoError(t, err, "garbage secret must be replaced with a usable keypair")
}

func TestEnsureSecretCert_LosesCreateRaceGracefully(t *testing.T) {
	winner := generateHostCert(t, testHosts, 365*24*time.Hour)
	winnerSecret := tlsSecret(t, winner)

	client := kubefake.NewSimpleClientset()
	client.PrependReactor("create", "secrets", func(kubetesting.Action) (bool, runtime.Object, error) {
		// Simulate another replica winning the create race: the secret pops
		// into existence, and our create fails with AlreadyExists.
		_, err := client.Tracker().Get(corev1.SchemeGroupVersion.WithResource("secrets"), testNamespace, TLSSecretName)
		if apierrors.IsNotFound(err) {
			require.NoError(t, client.Tracker().Add(winnerSecret))
		}
		return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, TLSSecretName)
	})

	cert, err := ensureSecretCert(t.Context(), client, testNamespace, testHosts)
	require.NoError(t, err)

	assert.Equal(t, winner.Certificate, cert.Certificate, "loser of the create race must serve the winner's certificate")
}

func TestEnsureSecretCert_LosesUpdateRaceGracefully(t *testing.T) {
	expiring := generateHostCert(t, testHosts, 24*time.Hour)
	winner := generateHostCert(t, testHosts, 365*24*time.Hour)
	client := kubefake.NewSimpleClientset(tlsSecret(t, expiring))

	client.PrependReactor("update", "secrets", func(kubetesting.Action) (bool, runtime.Object, error) {
		// Another replica rotated first; our update conflicts and the secret
		// now holds the winner's keypair.
		winnerSecret := tlsSecret(t, winner)
		require.NoError(t, client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("secrets"), winnerSecret, testNamespace))
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, TLSSecretName, nil)
	})

	cert, err := ensureSecretCert(t.Context(), client, testNamespace, testHosts)
	require.NoError(t, err)

	assert.Equal(t, winner.Certificate, cert.Certificate, "loser of the update race must serve the winner's certificate")
}

func TestLoadServingCert_PrefersMountedFiles(t *testing.T) {
	mounted := generateHostCert(t, testHosts, 365*24*time.Hour)
	certPEM, keyPEM := tlsutil.EncodeX509KeyPair(*mounted)
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))

	client := kubefake.NewSimpleClientset()
	cert, err := LoadServingCert(t.Context(), certPath, keyPath, testHosts, client, testNamespace)
	require.NoError(t, err)

	assert.Equal(t, mounted.Certificate, cert.Certificate)
	assert.Empty(t, client.Actions(), "mounted certificate must not touch the API")
}

func TestLoadServingCert_FallsBackToSecret(t *testing.T) {
	client := kubefake.NewSimpleClientset()

	cert, err := LoadServingCert(t.Context(), "/nonexistent/tls.crt", "/nonexistent/tls.key", testHosts, client, testNamespace)
	require.NoError(t, err)
	require.NotNil(t, cert.Leaf)

	getTLSSecret(t, client)
}

func TestLoadServingCert_EphemeralWithoutCluster(t *testing.T) {
	cert, err := LoadServingCert(t.Context(), "", "", testHosts, nil, testNamespace)
	require.NoError(t, err)
	require.NotNil(t, cert.Leaf)
	require.NoError(t, cert.Leaf.VerifyHostname("localhost"))
}
