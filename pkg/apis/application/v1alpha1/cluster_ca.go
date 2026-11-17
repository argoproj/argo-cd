package v1alpha1

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
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

var systemRootCADirs = []string{
	"/etc/ssl/certs",
	"/etc/pki/tls/certs",
}

// systemRootCAsPEM returns the PEM encoded system root CAs. Like Go's own system pool, they are read once per process.
var systemRootCAsPEM = sync.OnceValue(loadSystemRootCAsPEM)

func loadSystemRootCAsPEM() []byte {
	files, dirs := systemRootCAFiles, systemRootCADirs
	if file := os.Getenv("SSL_CERT_FILE"); file != "" {
		files = []string{file}
	}
	if dir := os.Getenv("SSL_CERT_DIR"); dir != "" {
		dirs = filepath.SplitList(dir)
	}
	roots := &pemCertSet{seen: map[string]bool{}}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err == nil {
			roots.add(data)
			break
		}
		if !os.IsNotExist(err) {
			log.Warnf("Failed to read system root CAs from %s: %v", file, err)
		}
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Warnf("Failed to read system root CAs from %s: %v", dir, err)
			}
			continue
		}
		for _, entry := range entries {
			if data, err := os.ReadFile(filepath.Join(dir, entry.Name())); err == nil {
				roots.add(data)
			}
		}
	}
	if len(roots.pem) == 0 {
		log.Warnf("No system root CAs found in %v or %v: clusters trusting both the system roots and the default cluster CA bundle will trust only that bundle", files, dirs)
		return nil
	}
	return roots.pem
}

type pemCertSet struct {
	seen map[string]bool
	pem  []byte
}

func (s *pemCertSet) add(data []byte) {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 || s.seen[string(block.Bytes)] {
			continue
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			continue
		}
		s.seen[string(block.Bytes)] = true
		s.pem = append(s.pem, pem.EncodeToMemory(block)...)
	}
}

type defaultCABundleTrust struct {
	bundle []byte
	caData []byte
	pool   *x509.CertPool
}

var defaultCABundleTrustCache struct {
	sync.Mutex
	bundleOnly      *defaultCABundleTrust
	withSystemRoots *defaultCABundleTrust
}

func trustForDefaultCABundle(bundle []byte, withSystemRoots bool) *defaultCABundleTrust {
	defaultCABundleTrustCache.Lock()
	defer defaultCABundleTrustCache.Unlock()
	cached := &defaultCABundleTrustCache.bundleOnly
	if withSystemRoots {
		cached = &defaultCABundleTrustCache.withSystemRoots
	}
	if trust := *cached; trust != nil && bytes.Equal(trust.bundle, bundle) {
		return trust
	}
	caData := bytes.Clone(bundle)
	if withSystemRoots {
		roots := systemRootCAsPEM()
		caData = make([]byte, 0, len(roots)+1+len(bundle))
		caData = append(caData, roots...)
		caData = append(caData, '\n')
		caData = append(caData, bundle...)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caData)
	trust := &defaultCABundleTrust{bundle: bytes.Clone(bundle), caData: caData, pool: pool}
	*cached = trust
	return trust
}

// Validate returns an error if t is not one of the supported values
func (t ClusterCATrust) Validate() error {
	switch t {
	case "", ClusterCATrustBundle, ClusterCATrustSystem, ClusterCATrustBundleAndSystem:
		return nil
	}
	return fmt.Errorf("invalid caTrust %q: must be one of %q, %q or %q", string(t),
		ClusterCATrustBundle, ClusterCATrustSystem, ClusterCATrustBundleAndSystem)
}

// DependsOnDefaultCABundle returns true if the CAs the cluster trusts change with the default cluster CA bundle
func (c *Cluster) DependsOnDefaultCABundle() bool {
	return c.Server != KubernetesInternalAPIServerAddr && len(c.Config.CAData) == 0 && !c.Config.Insecure &&
		c.Config.CATrust != ClusterCATrustSystem
}

func (c *Cluster) defaultCABundleTrust() (*defaultCABundleTrust, error) {
	if c.Server == KubernetesInternalAPIServerAddr {
		return nil, nil
	}
	if err := c.Config.CATrust.Validate(); err != nil {
		return nil, err
	}
	if !c.DependsOnDefaultCABundle() {
		return nil, nil
	}
	if len(c.DefaultCABundle) == 0 {
		if c.Config.CATrust == ClusterCATrustBundle {
			return nil, fmt.Errorf("caTrust is %q but there is no CA bundle in the %s key of the %s ConfigMap",
				string(ClusterCATrustBundle), common.ArgoCDClusterCAConfigMapKey, common.ArgoCDClusterCAConfigMapName)
		}
		return nil, nil
	}
	return trustForDefaultCABundle(c.DefaultCABundle, c.Config.CATrust == ClusterCATrustBundleAndSystem), nil
}
