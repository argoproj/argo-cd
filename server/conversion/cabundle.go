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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
)

// ReconcileCRDConversionConfig points the Application CRD's conversion
// webhook at this webhook instance: it fixes the service reference to this
// pod's actual service name and namespace, and injects the CA bundle from the
// server's TLS certificate. This makes the webhook work out of the box with
// self-signed certificates and with installations into any namespace — the
// shipped CRD hardcodes codegen-time defaults (argocd-conversion-webhook in
// namespace argocd), which a kustomize namespace transformer does not rewrite
// because the CRD is cluster-scoped.
//
// This function is safe to call on every startup — it only patches the CRD
// when the conversion webhook is configured and something needs updating.
func ReconcileCRDConversionConfig(ctx context.Context, restConfig *rest.Config, tlsCert *tls.Certificate, serviceName, namespace string) error {
	// Create apiextensions client
	apiextClient, err := apiextensionsclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create apiextensions client: %w", err)
	}

	return reconcileCRDConversionConfig(ctx, apiextClient, tlsCert, serviceName, namespace)
}

func reconcileCRDConversionConfig(ctx context.Context, apiextClient apiextensionsclient.Interface, tlsCert *tls.Certificate, serviceName, namespace string) error {
	// Get current CRD to check if conversion webhook is configured
	crd, err := apiextClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, application.ApplicationFullName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Application CRD: %w", err)
	}

	// Check if conversion webhook is configured
	if crd.Spec.Conversion == nil || crd.Spec.Conversion.Strategy != "Webhook" {
		log.Debug("Application CRD does not use webhook conversion, skipping conversion config reconciliation")
		return nil
	}

	// clientConfig accumulates the fields that need fixing; empty means the
	// CRD already matches this instance and no patch is issued.
	clientConfig := map[string]any{}

	var currentClientConfig *apiextensionsv1.WebhookClientConfig
	if crd.Spec.Conversion.Webhook != nil {
		currentClientConfig = crd.Spec.Conversion.Webhook.ClientConfig
	}

	// The service reference must point at this pod's service or the apiserver
	// routes conversion requests into the void. The pod knows its own service
	// name (flag/env) and namespace (downward API), so it is the authority.
	if serviceName != "" && namespace != "" {
		current := &apiextensionsv1.ServiceReference{}
		if currentClientConfig != nil && currentClientConfig.Service != nil {
			current = currentClientConfig.Service
		}
		if current.Name != serviceName || current.Namespace != namespace {
			log.Infof("Application CRD conversion webhook service is %s/%s, updating it to %s/%s",
				current.Namespace, current.Name, namespace, serviceName)
			// A merge patch of just these keys preserves the existing path and port.
			clientConfig["service"] = map[string]any{
				"name":      serviceName,
				"namespace": namespace,
			}
		}
	}

	if caPEM, err := neededCABundle(currentClientConfig, tlsCert); err != nil {
		return err
	} else if caPEM != nil {
		// json.Marshal base64-encodes a []byte field, which matches
		// Kubernetes' wire format for caBundle.
		clientConfig["caBundle"] = caPEM
	}

	if len(clientConfig) == 0 {
		return nil
	}

	patch, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"conversion": map[string]any{
				"webhook": map[string]any{
					"clientConfig": clientConfig,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to build conversion config patch: %w", err)
	}

	_, err = apiextClient.ApiextensionsV1().CustomResourceDefinitions().Patch(
		ctx,
		application.ApplicationFullName,
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch CRD conversion config: %w", err)
	}

	log.Info("Successfully reconciled Application CRD conversion webhook config")
	return nil
}

// neededCABundle returns the CA bundle the CRD should carry for the given
// serving certificate, or nil when no update is needed (no certificate to
// derive a CA from, or the existing bundle already verifies it).
func neededCABundle(clientConfig *apiextensionsv1.WebhookClientConfig, tlsCert *tls.Certificate) ([]byte, error) {
	if tlsCert == nil || len(tlsCert.Certificate) == 0 {
		log.Debug("No TLS certificate available, skipping CA bundle injection")
		return nil, nil
	}

	// Get the CA certificate (for self-signed certs, the cert itself is the CA)
	// For cert chains, use the last cert (typically the CA)
	caCertDER := tlsCert.Certificate[len(tlsCert.Certificate)-1]

	// Parse to verify it's valid
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// Leave an existing caBundle alone when it can verify the certificate we
	// are about to serve (cert-manager's CA injector or a manual install
	// manages it, and the mounted serving cert chains to it). Otherwise merge
	// our CA into the bundle rather than replacing it: during a certificate
	// rotation, pods serving the previous certificate keep working as long as
	// the previous CA stays in the bundle. Expired entries are dropped on the
	// way, so the bundle only accumulates CAs that can still verify something.
	var existingBundle []byte
	if clientConfig != nil && len(clientConfig.CABundle) > 0 {
		existingBundle = clientConfig.CABundle
		if caBundleVerifiesCert(existingBundle, tlsCert) {
			log.Info("Existing CA bundle on Application CRD verifies the serving certificate; leaving it unchanged")
			return nil, nil
		}
		log.Info("Existing CA bundle on Application CRD does not verify the serving certificate; merging our CA into it")
	}
	return mergeCABundle(existingBundle, caCert), nil
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
