package v1alpha1

import (
	"bytes"
	"crypto/x509"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
)

// systemRootCAFiles are the CA bundle files Go reads its system roots from on Linux, in the same order of preference.
var systemRootCAFiles = []string{
	"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu/Gentoo etc.
	"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
	"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
	"/etc/pki/tls/cacert.pem",                           // OpenELEC
	"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
	"/etc/ssl/cert.pem",                                 // Alpine Linux, macOS
}

// systemRootCAsPEM returns the PEM encoded system root CAs. Like Go's own system pool, they are read once per process.
var systemRootCAsPEM = sync.OnceValue(loadSystemRootCAsPEM)

func loadSystemRootCAsPEM() []byte {
	files := systemRootCAFiles
	if file := os.Getenv("SSL_CERT_FILE"); file != "" {
		files = []string{file}
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err == nil {
			return data
		}
		if !os.IsNotExist(err) {
			log.Warnf("Failed to read system root CAs from %s: %v", file, err)
		}
	}
	log.Warnf("No system root CA file found in %v: clusters using the default cluster CA bundle will trust only that bundle", files)
	return nil
}

type defaultCABundleTrust struct {
	bundle []byte
	caData []byte
	pool   *x509.CertPool
}

var defaultCABundleTrustCache struct {
	sync.Mutex
	trust *defaultCABundleTrust
}

func trustForDefaultCABundle(bundle []byte) *defaultCABundleTrust {
	defaultCABundleTrustCache.Lock()
	defer defaultCABundleTrustCache.Unlock()
	if trust := defaultCABundleTrustCache.trust; trust != nil && bytes.Equal(trust.bundle, bundle) {
		return trust
	}
	roots := systemRootCAsPEM()
	caData := make([]byte, 0, len(roots)+1+len(bundle))
	caData = append(caData, roots...)
	caData = append(caData, '\n')
	caData = append(caData, bundle...)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caData)
	trust := &defaultCABundleTrust{bundle: bytes.Clone(bundle), caData: caData, pool: pool}
	defaultCABundleTrustCache.trust = trust
	return trust
}

func (c *Cluster) usesDefaultCABundle() bool {
	return c.Server != KubernetesInternalAPIServerAddr && len(c.DefaultCABundle) > 0 &&
		len(c.Config.CAData) == 0 && !c.Config.Insecure
}
