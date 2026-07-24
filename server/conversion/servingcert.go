package conversion

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	tlsutil "github.com/argoproj/argo-cd/v3/util/tls"
)

// TLSSecretName is the Secret the webhook persists its generated serving
// certificate in. It matches the optional TLS volume in the webhook's
// Deployment manifest, so a keypair created here is what later pods find
// mounted at the default cert/key paths.
const TLSSecretName = "argocd-conversion-webhook-tls"

// renewalMargin is how long before expiry a stored certificate is rotated on
// pod startup. Rotation only happens at startup, so the margin is generous:
// any pod restart within the last 30 days of validity refreshes the keypair.
const renewalMargin = 30 * 24 * time.Hour

// LoadServingCert resolves the certificate the webhook serves, in order of
// preference:
//
//  1. The mounted cert/key files (user-provided or cert-manager managed).
//  2. A keypair persisted in the TLSSecretName Secret, created on first use.
//  3. An ephemeral in-memory keypair when no cluster access is available
//     (local development).
//
// The Secret-backed path is what makes the default install safe across pod
// restarts, rolling updates, and replicas > 1: every pod serves the same
// certificate, so the CA bundle on the CRD stays valid for all of them. A
// per-pod ephemeral certificate would instead break every conversion served
// by any pod other than the one that patched the CRD last.
func LoadServingCert(ctx context.Context, certPath, keyPath string, hosts []string, kubeClient kubernetes.Interface, namespace string) (*tls.Certificate, error) {
	if fileExists(certPath) && fileExists(keyPath) {
		log.Infof("Loading TLS configuration from cert=%s and key=%s", certPath, keyPath)
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("unable to load TLS certificate from cert=%s and key=%s: %w", certPath, keyPath, err)
		}
		if err := setLeaf(&cert); err != nil {
			return nil, err
		}
		return &cert, nil
	}

	if kubeClient != nil {
		return ensureSecretCert(ctx, kubeClient, namespace, hosts)
	}

	log.Info("No cluster access, generating ephemeral self-signed TLS certificate for this session")
	return generateServingCert(hosts)
}

// ensureSecretCert returns the keypair stored in the TLS Secret, creating or
// rotating it as needed. Concurrent pods race benignly: creation and update
// both fall back to re-reading the Secret, so every pod ends up serving the
// winner's certificate.
func ensureSecretCert(ctx context.Context, kubeClient kubernetes.Interface, namespace string, hosts []string) (*tls.Certificate, error) {
	secrets := kubeClient.CoreV1().Secrets(namespace)

	secret, err := secrets.Get(ctx, TLSSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		cert, err := createSecretCert(ctx, kubeClient, namespace, hosts)
		if err != nil {
			return nil, err
		}
		return cert, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS secret %s/%s: %w", namespace, TLSSecretName, err)
	}

	cert, reason := usableSecretCert(secret, hosts)
	if cert != nil {
		log.Infof("Using TLS certificate from secret %s/%s", namespace, TLSSecretName)
		return cert, nil
	}

	log.Infof("TLS certificate in secret %s/%s needs regeneration (%s)", namespace, TLSSecretName, reason)
	newCert, err := generateServingCert(hosts)
	if err != nil {
		return nil, err
	}
	certPEM, keyPEM := tlsutil.EncodeX509KeyPair(*newCert)
	secret.Type = corev1.SecretTypeTLS
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       certPEM,
		corev1.TLSPrivateKeyKey: keyPEM,
	}
	// Optimistic-concurrency update: on conflict another pod rotated first,
	// so serve its certificate instead of fighting over the Secret.
	_, err = secrets.Update(ctx, secret, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		return readSecretCert(ctx, kubeClient, namespace)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update TLS secret %s/%s: %w", namespace, TLSSecretName, err)
	}
	log.Infof("Rotated TLS certificate in secret %s/%s", namespace, TLSSecretName)
	return newCert, nil
}

func createSecretCert(ctx context.Context, kubeClient kubernetes.Interface, namespace string, hosts []string) (*tls.Certificate, error) {
	cert, err := generateServingCert(hosts)
	if err != nil {
		return nil, err
	}
	certPEM, keyPEM := tlsutil.EncodeX509KeyPair(*cert)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TLSSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    "argocd-conversion-webhook",
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certPEM,
			corev1.TLSPrivateKeyKey: keyPEM,
		},
	}
	_, err = kubeClient.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		// Another pod created it first; serve that certificate.
		return readSecretCert(ctx, kubeClient, namespace)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS secret %s/%s: %w", namespace, TLSSecretName, err)
	}
	log.Infof("Persisted generated TLS certificate in secret %s/%s", namespace, TLSSecretName)
	return cert, nil
}

// readSecretCert reads the Secret written by a concurrent pod. Unlike
// ensureSecretCert it never writes: the other pod just did, so an unusable
// certificate here is an error rather than a rotation trigger.
func readSecretCert(ctx context.Context, kubeClient kubernetes.Interface, namespace string) (*tls.Certificate, error) {
	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, TLSSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS secret %s/%s after losing write race: %w", namespace, TLSSecretName, err)
	}
	cert, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return nil, fmt.Errorf("TLS secret %s/%s written by concurrent pod is not usable: %w", namespace, TLSSecretName, err)
	}
	if err := setLeaf(&cert); err != nil {
		return nil, err
	}
	log.Infof("Using TLS certificate written concurrently to secret %s/%s", namespace, TLSSecretName)
	return &cert, nil
}

// usableSecretCert parses the stored keypair and reports why it cannot be
// served if it can't be: unparseable, expired or within the renewal margin,
// or missing a required host (e.g. the webhook Service was renamed or moved
// to another namespace since the certificate was issued).
func usableSecretCert(secret *corev1.Secret, hosts []string) (*tls.Certificate, string) {
	cert, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return nil, fmt.Sprintf("keypair does not parse: %v", err)
	}
	if err := setLeaf(&cert); err != nil {
		return nil, err.Error()
	}
	now := time.Now()
	if now.Before(cert.Leaf.NotBefore) {
		return nil, "certificate is not yet valid"
	}
	if now.After(cert.Leaf.NotAfter.Add(-renewalMargin)) {
		return nil, fmt.Sprintf("certificate expires %s, within the %s renewal margin", cert.Leaf.NotAfter.Format(time.RFC3339), renewalMargin)
	}
	for _, host := range hosts {
		if err := cert.Leaf.VerifyHostname(host); err != nil {
			return nil, fmt.Sprintf("certificate is not valid for host %q", host)
		}
	}
	return &cert, ""
}

func generateServingCert(hosts []string) (*tls.Certificate, error) {
	cert, err := tlsutil.GenerateX509KeyPair(tlsutil.CertOptions{
		Hosts:        hosts,
		Organization: "Argo CD",
		IsCA:         true,
	})
	if err != nil {
		return nil, fmt.Errorf("error generating serving certificate: %w", err)
	}
	return cert, nil
}

func setLeaf(cert *tls.Certificate) error {
	if cert.Leaf != nil || len(cert.Certificate) == 0 {
		return nil
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("error parsing certificate: %w", err)
	}
	cert.Leaf = leaf
	return nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warnf("could not read TLS file from %s: %v", path, err)
	}
	return err == nil
}
