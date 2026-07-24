package conversion

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
)

// InjectCABundle updates the Application CRD's conversion webhook with the CA bundle
// from the server's TLS certificate. This enables the conversion webhook to work
// out of the box with self-signed certificates.
//
// This function is safe to call on every server startup - it will only patch the CRD
// if the conversion webhook is configured and the CA bundle needs updating.
func InjectCABundle(ctx context.Context, restConfig *rest.Config, tlsCert *tls.Certificate) error {
	// Create apiextensions client
	apiextClient, err := apiextensionsclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create apiextensions client: %w", err)
	}

	return injectCABundle(ctx, apiextClient, tlsCert)
}

func injectCABundle(ctx context.Context, apiextClient apiextensionsclient.Interface, tlsCert *tls.Certificate) error {
	if tlsCert == nil || len(tlsCert.Certificate) == 0 {
		log.Debug("No TLS certificate available, skipping CA bundle injection")
		return nil
	}

	// Get current CRD to check if conversion webhook is configured
	crd, err := apiextClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, application.ApplicationFullName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Application CRD: %w", err)
	}

	// Check if conversion webhook is configured
	if crd.Spec.Conversion == nil || crd.Spec.Conversion.Strategy != "Webhook" {
		log.Debug("Application CRD does not use webhook conversion, skipping CA bundle injection")
		return nil
	}

	// Get the CA certificate (for self-signed certs, the cert itself is the CA)
	// For cert chains, use the last cert (typically the CA)
	caCertDER := tlsCert.Certificate[len(tlsCert.Certificate)-1]

	// Parse to verify it's valid
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Leave an existing caBundle alone when it can verify the certificate we
	// are about to serve (cert-manager's CA injector or a manual install
	// manages it, and the mounted serving cert chains to it). Otherwise merge
	// our CA into the bundle rather than replacing it: during a certificate
	// rotation, pods serving the previous certificate keep working as long as
	// the previous CA stays in the bundle. Expired entries are dropped on the
	// way, so the bundle only accumulates CAs that can still verify something.
	var existingBundle []byte
	if crd.Spec.Conversion.Webhook != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig != nil &&
		len(crd.Spec.Conversion.Webhook.ClientConfig.CABundle) > 0 {
		existingBundle = crd.Spec.Conversion.Webhook.ClientConfig.CABundle
		if caBundleVerifiesCert(existingBundle, tlsCert) {
			log.Info("Existing CA bundle on Application CRD verifies the serving certificate; leaving it unchanged")
			return nil
		}
		log.Info("Existing CA bundle on Application CRD does not verify the serving certificate; merging our CA into it")
	}
	caPEM := mergeCABundle(existingBundle, caCert)

	// Build the patch via json.Marshal — a []byte field is base64-encoded
	// by encoding/json, which matches Kubernetes' wire format for caBundle.
	patch, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"conversion": map[string]any{
				"webhook": map[string]any{
					"clientConfig": map[string]any{
						"caBundle": caPEM,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to build CA bundle patch: %w", err)
	}

	_, err = apiextClient.ApiextensionsV1().CustomResourceDefinitions().Patch(
		ctx,
		application.ApplicationFullName,
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch CRD with CA bundle: %w", err)
	}

	log.Info("Successfully injected CA bundle into Application CRD conversion webhook")
	return nil
}

// mergeCABundle returns a PEM bundle containing the still-valid certificates
// from the existing bundle followed by the given CA. Unparseable and expired
// entries are dropped; if the whole existing bundle is garbage, the result is
// just the new CA.
func mergeCABundle(existing []byte, caCert *x509.Certificate) []byte {
	var merged []byte
	now := time.Now()
	rest := existing
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil || now.After(cert.NotAfter) || bytes.Equal(cert.Raw, caCert.Raw) {
			continue
		}
		merged = append(merged, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})...)
	}
	return append(merged, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})...)
}

// caBundleVerifiesCert reports whether the PEM-encoded CA bundle can verify
// the leaf of the given serving certificate chain.
func caBundleVerifiesCert(bundlePEM []byte, cert *tls.Certificate) bool {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(bundlePEM) {
		return false
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return false
	}
	intermediates := x509.NewCertPool()
	for _, der := range cert.Certificate[1:] {
		ic, err := x509.ParseCertificate(der)
		if err != nil {
			return false
		}
		intermediates.AddCert(ic)
	}
	_, err = leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates})
	return err == nil
}
