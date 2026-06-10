package conversion

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"

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
	if tlsCert == nil || len(tlsCert.Certificate) == 0 {
		log.Debug("No TLS certificate available, skipping CA bundle injection")
		return nil
	}

	// Create apiextensions client
	apiextClient, err := apiextensionsclient.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create apiextensions client: %w", err)
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

	// Encode as PEM
	caPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCert.Raw,
	})

	// Only inject when caBundle is empty. If something else has already
	// populated it (cert-manager's CA injector, a manual install, etc),
	// leave it alone — overwriting would break those trust chains.
	if crd.Spec.Conversion.Webhook != nil &&
		crd.Spec.Conversion.Webhook.ClientConfig != nil &&
		len(crd.Spec.Conversion.Webhook.ClientConfig.CABundle) > 0 {
		log.Info("CA bundle is already set on Application CRD; leaving it unchanged")
		return nil
	}

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
