package tls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
)

const (
	DefaultRSABits = 2048
)

var (
	tlsVersionByString = map[string]uint16{
		"1.0": tls.VersionTLS10,
		"1.1": tls.VersionTLS11,
		"1.2": tls.VersionTLS12,
	}
)

type CertOptions struct {
	// Hostnames and IPs to generate a certificate for
	Hosts []string
	// Name of organization in certificate
	Organization string
	// Creation date
	ValidFrom time.Time
	// Duration that certificate is valid for
	ValidFor time.Duration
	// whether this cert should be its own Certificate Authority
	IsCA bool
	// Size of RSA key to generate. Ignored if --ecdsa-curve is set
	RSABits int
	// ECDSA curve to use to generate a key. Valid values are P224, P256 (recommended), P384, P521
	ECDSACurve string
}

type ConfigCustomizer = func(*tls.Config)

// BestEffortSystemCertPool returns system cert pool as best effort, otherwise an empty cert pool
func BestEffortSystemCertPool() *x509.CertPool {
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		return x509.NewCertPool()
	}
	return rootCAs
}

func getTLSVersionByString(version string) (uint16, error) {
	if version == "" {
		return 0, nil
	}
	if res, ok := tlsVersionByString[version]; ok {
		return res, nil
	}
	return 0, fmt.Errorf("%s is not valid TLS version", version)
}

func AddTLSFlagsToCmd(cmd *cobra.Command) func() (ConfigCustomizer, error) {
	minVersionStr := ""
	maxVersionStr := ""
	cmd.Flags().StringVar(&minVersionStr, "tlsminversion", "", "The minimum SSL/TLS version that is acceptable (one of: 1.0|1.1|1.2)")
	cmd.Flags().StringVar(&maxVersionStr, "tlsmaxversion", "", "The maximum SSL/TLS version that is acceptable (one of: 1.0|1.1|1.2)")

	return func() (ConfigCustomizer, error) {
		minVersion, err := getTLSVersionByString(minVersionStr)
		if err != nil {
			return nil, err
		}
		maxVersion, err := getTLSVersionByString(maxVersionStr)
		if err != nil {
			return nil, err
		}
		return func(config *tls.Config) {
			config.MinVersion = minVersion
			config.MaxVersion = maxVersion
		}, nil
	}
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func generate(opts CertOptions) ([]byte, crypto.PrivateKey, error) {
	if len(opts.Hosts) == 0 {
		return nil, nil, fmt.Errorf("hosts not supplied")
	}

	var privateKey crypto.PrivateKey
	var err error
	switch opts.ECDSACurve {
	case "":
		rsaBits := DefaultRSABits
		if opts.RSABits != 0 {
			rsaBits = opts.RSABits
		}
		privateKey, err = rsa.GenerateKey(rand.Reader, rsaBits)
	case "P224":
		privateKey, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		privateKey, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		privateKey, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		return nil, nil, fmt.Errorf("Unrecognized elliptic curve: %q", opts.ECDSACurve)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %s", err)
	}

	var notBefore time.Time
	if opts.ValidFrom.IsZero() {
		notBefore = time.Now()
	} else {
		notBefore = opts.ValidFrom
	}
	var validFor time.Duration
	if opts.ValidFor == 0 {
		validFor = 365 * 24 * time.Hour
	}
	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %s", err)
	}

	if opts.Organization == "" {
		return nil, nil, fmt.Errorf("organization not supplied")
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{opts.Organization},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for _, h := range opts.Hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if opts.IsCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(privateKey), privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create certificate: %s", err)
	}
	return certBytes, privateKey, nil
}

// generatePEM generates a new certificate and key and returns it as PEM encoded bytes
func generatePEM(opts CertOptions) ([]byte, []byte, error) {
	certBytes, privateKey, err := generate(opts)
	if err != nil {
		return nil, nil, err
	}
	certpem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keypem := pem.EncodeToMemory(pemBlockForKey(privateKey))
	return certpem, keypem, nil
}

// GenerateX509KeyPair generates a X509 key pair
func GenerateX509KeyPair(opts CertOptions) (*tls.Certificate, error) {
	certpem, keypem, err := generatePEM(opts)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certpem, keypem)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// EncodeX509KeyPair encodes a TLS Certificate into its pem encoded format for storage
func EncodeX509KeyPair(cert tls.Certificate) ([]byte, []byte) {

	certpem := []byte{}
	for _, certtmp := range cert.Certificate {
		certpem = append(certpem, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certtmp})...)
	}
	keypem := pem.EncodeToMemory(pemBlockForKey(cert.PrivateKey))
	return certpem, keypem
}

// EncodeX509KeyPairString encodes a TLS Certificate into its pem encoded string format
func EncodeX509KeyPairString(cert tls.Certificate) (string, string) {
	certpem, keypem := EncodeX509KeyPair(cert)
	return string(certpem), string(keypem)
}
