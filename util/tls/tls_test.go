package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var chain = `-----BEGIN CERTIFICATE-----
MIIG5jCCBc6gAwIBAgIQAze5KDR8YKauxa2xIX84YDANBgkqhkiG9w0BAQUFADBs
MQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3
d3cuZGlnaWNlcnQuY29tMSswKQYDVQQDEyJEaWdpQ2VydCBIaWdoIEFzc3VyYW5j
ZSBFViBSb290IENBMB4XDTA3MTEwOTEyMDAwMFoXDTIxMTExMDAwMDAwMFowaTEL
MAkGA1UEBhMCVVMxFTATBgNVBAoTDERpZ2lDZXJ0IEluYzEZMBcGA1UECxMQd3d3
LmRpZ2ljZXJ0LmNvbTEoMCYGA1UEAxMfRGlnaUNlcnQgSGlnaCBBc3N1cmFuY2Ug
RVYgQ0EtMTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAPOWYth1bhn/
PzR8SU8xfg0ETpmB4rOFVZEwscCvcLssqOcYqj9495BoUoYBiJfiOwZlkKq9ZXbC
7L4QWzd4g2B1Rca9dKq2n6Q6AVAXxDlpufFP74LByvNK28yeUE9NQKM6kOeGZrzw
PnYoTNF1gJ5qNRQ1A57bDIzCKK1Qss72kaPDpQpYSfZ1RGy6+c7pqzoC4E3zrOJ6
4GAiBTyC01Li85xH+DvYskuTVkq/cKs+6WjIHY9YHSpNXic9rQpZL1oRIEDZaARo
LfTAhAsKG3jf7RpY3PtBWm1r8u0c7lwytlzs16YDMqbo3rcoJ1mIgP97rYlY1R4U
pPKwcNSgPqcCAwEAAaOCA4UwggOBMA4GA1UdDwEB/wQEAwIBhjA7BgNVHSUENDAy
BggrBgEFBQcDAQYIKwYBBQUHAwIGCCsGAQUFBwMDBggrBgEFBQcDBAYIKwYBBQUH
AwgwggHEBgNVHSAEggG7MIIBtzCCAbMGCWCGSAGG/WwCATCCAaQwOgYIKwYBBQUH
AgEWLmh0dHA6Ly93d3cuZGlnaWNlcnQuY29tL3NzbC1jcHMtcmVwb3NpdG9yeS5o
dG0wggFkBggrBgEFBQcCAjCCAVYeggFSAEEAbgB5ACAAdQBzAGUAIABvAGYAIAB0
AGgAaQBzACAAQwBlAHIAdABpAGYAaQBjAGEAdABlACAAYwBvAG4AcwB0AGkAdAB1
AHQAZQBzACAAYQBjAGMAZQBwAHQAYQBuAGMAZQAgAG8AZgAgAHQAaABlACAARABp
AGcAaQBDAGUAcgB0ACAARQBWACAAQwBQAFMAIABhAG4AZAAgAHQAaABlACAAUgBl
AGwAeQBpAG4AZwAgAFAAYQByAHQAeQAgAEEAZwByAGUAZQBtAGUAbgB0ACAAdwBo
AGkAYwBoACAAbABpAG0AaQB0ACAAbABpAGEAYgBpAGwAaQB0AHkAIABhAG4AZAAg
AGEAcgBlACAAaQBuAGMAbwByAHAAbwByAGEAdABlAGQAIABoAGUAcgBlAGkAbgAg
AGIAeQAgAHIAZQBmAGUAcgBlAG4AYwBlAC4wEgYDVR0TAQH/BAgwBgEB/wIBADCB
gwYIKwYBBQUHAQEEdzB1MCQGCCsGAQUFBzABhhhodHRwOi8vb2NzcC5kaWdpY2Vy
dC5jb20wTQYIKwYBBQUHMAKGQWh0dHA6Ly93d3cuZGlnaWNlcnQuY29tL0NBQ2Vy
dHMvRGlnaUNlcnRIaWdoQXNzdXJhbmNlRVZSb290Q0EuY3J0MIGPBgNVHR8EgYcw
gYQwQKA+oDyGOmh0dHA6Ly9jcmwzLmRpZ2ljZXJ0LmNvbS9EaWdpQ2VydEhpZ2hB
c3N1cmFuY2VFVlJvb3RDQS5jcmwwQKA+oDyGOmh0dHA6Ly9jcmw0LmRpZ2ljZXJ0
LmNvbS9EaWdpQ2VydEhpZ2hBc3N1cmFuY2VFVlJvb3RDQS5jcmwwHQYDVR0OBBYE
FExYyyXwQU9S9CjIgUObpqig5pLlMB8GA1UdIwQYMBaAFLE+w2kD+L9HAdSYJhoI
Au9jZCvDMA0GCSqGSIb3DQEBBQUAA4IBAQBMeheHKF0XvLIyc7/NLvVYMR3wsXFU
nNabZ5PbLwM+Fm8eA8lThKNWYB54lBuiqG+jpItSkdfdXJW777UWSemlQk808kf/
roF/E1S3IMRwFcuBCoHLdFfcnN8kpCkMGPAc5K4HM+zxST5Vz25PDVR708noFUjU
xbvcNRx3RQdIRYW9135TuMAW2ZXNi419yWBP0aKb49Aw1rRzNubS+QOy46T15bg+
BEkAui6mSnKDcp33C4ypieez12Qf1uNgywPE3IjpnSUBAHHLA7QpYCWP+UbRe3Gu
zVMSW4SOwg/H7ZMZ2cn6j1g0djIvruFQFGHUqFijyDATI+/GJYw2jxyA
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIDxTCCAq2gAwIBAgIQAqxcJmoLQJuPC3nyrkYldzANBgkqhkiG9w0BAQUFADBs
MQswCQYDVQQGEwJVUzEVMBMGA1UEChMMRGlnaUNlcnQgSW5jMRkwFwYDVQQLExB3
d3cuZGlnaWNlcnQuY29tMSswKQYDVQQDEyJEaWdpQ2VydCBIaWdoIEFzc3VyYW5j
ZSBFViBSb290IENBMB4XDTA2MTExMDAwMDAwMFoXDTMxMTExMDAwMDAwMFowbDEL
MAkGA1UEBhMCVVMxFTATBgNVBAoTDERpZ2lDZXJ0IEluYzEZMBcGA1UECxMQd3d3
LmRpZ2ljZXJ0LmNvbTErMCkGA1UEAxMiRGlnaUNlcnQgSGlnaCBBc3N1cmFuY2Ug
RVYgUm9vdCBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMbM5XPm
+9S75S0tMqbf5YE/yc0lSbZxKsPVlDRnogocsF9ppkCxxLeyj9CYpKlBWTrT3JTW
PNt0OKRKzE0lgvdKpVMSOO7zSW1xkX5jtqumX8OkhPhPYlG++MXs2ziS4wblCJEM
xChBVfvLWokVfnHoNb9Ncgk9vjo4UFt3MRuNs8ckRZqnrG0AFFoEt7oT61EKmEFB
Ik5lYYeBQVCmeVyJ3hlKV9Uu5l0cUyx+mM0aBhakaHPQNAQTXKFx01p8VdteZOE3
hzBWBOURtCmAEvF5OYiiAhF8J2a3iLd48soKqDirCmTCv2ZdlYTBoSUeh10aUAsg
EsxBu24LUTi4S8sCAwEAAaNjMGEwDgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQF
MAMBAf8wHQYDVR0OBBYEFLE+w2kD+L9HAdSYJhoIAu9jZCvDMB8GA1UdIwQYMBaA
FLE+w2kD+L9HAdSYJhoIAu9jZCvDMA0GCSqGSIb3DQEBBQUAA4IBAQAcGgaX3Nec
nzyIZgYIVyHbIUf4KmeqvxgydkAQV8GK83rZEWWONfqe/EW1ntlMMUu4kehDLI6z
eM7b41N5cdblIZQB2lWHmiRk9opmzN6cN82oNLFpmyPInngiK3BD41VHMWEZ71jF
hS9OMPagMRYjyOfiZRYzy78aG6A9+MpeizGLYAiJLQwGXFK3xPkKmNEVX58Svnw2
Yzi9RKR/5CYrCsSXaQ3pjOLAEFe4yHYSkVXySGnYvCoCWw9E1CAx2/S6cCZdkGCe
vEsXCS+0yx5DaMkHJ8HSXPfqIbloEpw8nL+e/IBcm2PN7EeqJSdnoDfzAIJ9VNep
+OkuE6N36B9K
-----END CERTIFICATE-----`

var privateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIBsAIBAAKBgQCgF35rHhOWi9+r4n9xM/ejvMEsQ8h6lams962k4U0WSdfySUev
hyI1bd3FRIb5fFqSBt6qPTiiiIw0KXte5dANB6lPe6HdUPTA/U4xHWi2FB/BfAyP
sOlUBfFp6dtkEEcEKt+Z8KTJYJEerRie24y+nsfZMnLBst6tsEBfx/U75wIBAwKB
gGq6VEdpYmRdHGzsbmP7vDiYe2zYHLwQ0AKnPKNErq6KQyQC5eEngbgT4WpWl+J2
Xn+R9m0vwNbaiDam0uD3p5192BaN2tdaW5P5JjfGa95ytRBCQ/cr+z03FjG9C6zQ
QZG5eyOoMloHAfnYiJMV5SZarfTiF9BGFvtcfrjhbterAgkDBMoUFjHxL0ECeDUI
f9nbOl1O2AgI/51gfHGo/NKv+kcQenM8RO7dy9+hUAulwqMlyszSq+0GdZdgQL/i
Lz8NclSgyuUtptmaSWtjB5Tdc8boaBApGKac7vB4M1AfTkng1+SplKbkdFlCVg4n
6EvCOrUFFsLp308JSbkv2240Q93JJwIJAgMxYrl2oMorAgcDNY7r7ttvAggOb9tA
6WMDHQ==
-----END RSA PRIVATE KEY-----`

func decodePem(certInput string) tls.Certificate {
	var cert tls.Certificate
	certPEMBlock := []byte(certInput)
	var certDERBlock *pem.Block
	for {
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert.Certificate = append(cert.Certificate, certDERBlock.Bytes)
		}
	}

	var keyDERBlock *pem.Block
	keyPEMBlock := []byte(privateKey)
	keyDERBlock, _ = pem.Decode(keyPEMBlock)
	cert.PrivateKey, _ = x509.ParsePKCS1PrivateKey(keyDERBlock.Bytes)
	return cert
}

func TestEncodeX509KeyPairString(t *testing.T) {
	certChain := decodePem(chain)
	cert, _ := EncodeX509KeyPairString(certChain)

	if strings.TrimSpace(chain) != strings.TrimSpace(cert) {
		t.Errorf("Incorrect, got: %s, want: %s", cert, chain)
	}

}

func TestGetTLSVersionByString(t *testing.T) {
	t.Run("Valid versions", func(t *testing.T) {
		for k, v := range tlsVersionByString {
			r, err := getTLSVersionByString(k)
			assert.NoError(t, err)
			assert.Equal(t, v, r)
		}
	})

	t.Run("Invalid versions", func(t *testing.T) {
		_, err := getTLSVersionByString("1.4")
		assert.Error(t, err)
	})

	t.Run("Empty versions", func(t *testing.T) {
		r, err := getTLSVersionByString("")
		assert.NoError(t, err)
		assert.Equal(t, r, uint16(0))
	})
}

func TestGetTLSCipherSuitesByString(t *testing.T) {
	suites := make([]string, 0)
	for _, s := range tls.CipherSuites() {
		t.Run(fmt.Sprintf("Test for valid suite %s", s.Name), func(t *testing.T) {
			ids, err := getTLSCipherSuitesByString(s.Name)
			assert.NoError(t, err)
			assert.Len(t, ids, 1)
			assert.Equal(t, s.ID, ids[0])
			suites = append(suites, s.Name)
		})
	}

	t.Run("Test colon separated list", func(t *testing.T) {
		ids, err := getTLSCipherSuitesByString(strings.Join(suites, ":"))
		assert.NoError(t, err)
		assert.Len(t, ids, len(suites))
	})

	suites = append([]string{"invalid"}, suites...)
	t.Run("Test invalid values", func(t *testing.T) {
		_, err := getTLSCipherSuitesByString(strings.Join(suites, ":"))
		assert.Error(t, err)
	})

}

func TestTLSVersionToString(t *testing.T) {
	t.Run("Test known versions", func(t *testing.T) {
		versions := make([]uint16, 0)
		for _, v := range tlsVersionByString {
			versions = append(versions, v)
		}
		s := tlsVersionsToStr(versions)
		assert.Len(t, s, len(versions))
	})
	t.Run("Test unknown version", func(t *testing.T) {
		s := tlsVersionsToStr([]uint16{999})
		assert.Len(t, s, 1)
		assert.Equal(t, "unknown", s[0])
	})
}

func TestGenerate(t *testing.T) {
	t.Run("Invalid: No hosts specified", func(t *testing.T) {
		opts := CertOptions{Hosts: []string{}, Organization: "Acme", ValidFrom: time.Now(), ValidFor: 10 * time.Hour}
		_, _, err := generate(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hosts not supplied")
	})

	t.Run("Invalid: No organization specified", func(t *testing.T) {
		opts := CertOptions{Hosts: []string{"localhost"}, Organization: "", ValidFrom: time.Now(), ValidFor: 10 * time.Hour}
		_, _, err := generate(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organization not supplied")
	})

	t.Run("Invalid: Unsupported curve specified", func(t *testing.T) {
		opts := CertOptions{Hosts: []string{"localhost"}, Organization: "Acme", ECDSACurve: "Curve?", ValidFrom: time.Now(), ValidFor: 10 * time.Hour}
		_, _, err := generate(opts)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Unrecognized elliptic curve")
	})

	for _, curve := range []string{"P224", "P256", "P384", "P521"} {
		t.Run(fmt.Sprintf("Create certificate with curve %s", curve), func(t *testing.T) {
			opts := CertOptions{Hosts: []string{"localhost"}, Organization: "Acme", ECDSACurve: curve}
			_, _, err := generate(opts)
			assert.NoError(t, err)
		})
	}

	t.Run("Create certificate with default options", func(t *testing.T) {
		opts := CertOptions{Hosts: []string{"localhost"}, Organization: "Acme"}
		certBytes, privKey, err := generate(opts)
		assert.NoError(t, err)
		assert.NotNil(t, privKey)
		cert, err := x509.ParseCertificate(certBytes)
		assert.NoError(t, err)
		assert.NotNil(t, cert)
		assert.Len(t, cert.DNSNames, 1)
		assert.Equal(t, "localhost", cert.DNSNames[0])
		assert.Empty(t, cert.IPAddresses)
		assert.LessOrEqual(t, int64(time.Since(cert.NotBefore)), int64(10*time.Second))
	})

	t.Run("Create certificate with IP ", func(t *testing.T) {
		opts := CertOptions{Hosts: []string{"localhost", "127.0.0.1"}, Organization: "Acme"}
		certBytes, privKey, err := generate(opts)
		assert.NoError(t, err)
		assert.NotNil(t, privKey)
		cert, err := x509.ParseCertificate(certBytes)
		assert.NoError(t, err)
		assert.NotNil(t, cert)
		assert.Len(t, cert.DNSNames, 1)
		assert.Equal(t, "localhost", cert.DNSNames[0])
		assert.Equal(t, "Acme", cert.Subject.Organization[0])
		assert.Len(t, cert.IPAddresses, 1)
		assert.Equal(t, "127.0.0.1", cert.IPAddresses[0].String())
	})

	t.Run("Create certificate with specific validity timeframe", func(t *testing.T) {
		opts := CertOptions{Hosts: []string{"localhost"}, Organization: "Acme", ValidFrom: time.Now().Add(1 * time.Hour)}
		certBytes, privKey, err := generate(opts)
		assert.NoError(t, err)
		assert.NotNil(t, privKey)
		cert, err := x509.ParseCertificate(certBytes)
		assert.NoError(t, err)
		assert.NotNil(t, cert)
		assert.GreaterOrEqual(t, (time.Now().Unix())+int64(1*time.Hour), cert.NotBefore.Unix())
	})
}

func TestGeneratePEM(t *testing.T) {
	t.Run("Invalid - PEM creation failure", func(t *testing.T) {
		opts := CertOptions{Hosts: nil, Organization: "Acme"}
		cert, key, err := generatePEM(opts)
		assert.Error(t, err)
		assert.Nil(t, cert)
		assert.Nil(t, key)
	})

	t.Run("Create PEM from certficate options", func(t *testing.T) {
		opts := CertOptions{Hosts: []string{"localhost"}, Organization: "Acme"}
		cert, key, err := generatePEM(opts)
		assert.NoError(t, err)
		assert.NotNil(t, cert)
		assert.NotNil(t, key)
	})

	t.Run("Create X509KeyPair", func(t *testing.T) {
		opts := CertOptions{Hosts: []string{"localhost"}, Organization: "Acme"}
		cert, err := GenerateX509KeyPair(opts)
		assert.NoError(t, err)
		assert.NotNil(t, cert)
	})
}

func TestGetTLSConfigCustomizer(t *testing.T) {
	t.Run("Valid TLS customization", func(t *testing.T) {
		cfunc, err := getTLSConfigCustomizer(DefaultTLSMinVersion, DefaultTLSMaxVersion, DefaultTLSCipherSuite)
		assert.NoError(t, err)
		assert.NotNil(t, cfunc)
		config := tls.Config{}
		cfunc(&config)
		assert.Equal(t, config.MinVersion, uint16(tls.VersionTLS12))
		assert.Equal(t, config.MaxVersion, uint16(tls.VersionTLS13))
	})

	t.Run("Valid TLS customization - No cipher customization for TLSv1.3 only with default ciphers", func(t *testing.T) {
		cfunc, err := getTLSConfigCustomizer("1.3", "1.3", DefaultTLSCipherSuite)
		assert.NoError(t, err)
		assert.NotNil(t, cfunc)
		config := tls.Config{}
		cfunc(&config)
		assert.Equal(t, config.MinVersion, uint16(tls.VersionTLS13))
		assert.Equal(t, config.MaxVersion, uint16(tls.VersionTLS13))
		assert.Len(t, config.CipherSuites, 0)
	})

	t.Run("Valid TLS customization - No cipher customization for TLSv1.3 only with custom ciphers", func(t *testing.T) {
		cfunc, err := getTLSConfigCustomizer("1.3", "1.3", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256")
		assert.NoError(t, err)
		assert.NotNil(t, cfunc)
		config := tls.Config{}
		cfunc(&config)
		assert.Equal(t, config.MinVersion, uint16(tls.VersionTLS13))
		assert.Equal(t, config.MaxVersion, uint16(tls.VersionTLS13))
		assert.Len(t, config.CipherSuites, 0)
	})

	t.Run("Invalid TLS customization - Min version higher than max version", func(t *testing.T) {
		cfunc, err := getTLSConfigCustomizer("1.3", "1.2", DefaultTLSCipherSuite)
		assert.Error(t, err)
		assert.Nil(t, cfunc)
	})

	t.Run("Invalid TLS customization - Invalid min version given", func(t *testing.T) {
		cfunc, err := getTLSConfigCustomizer("2.0", "1.2", DefaultTLSCipherSuite)
		assert.Error(t, err)
		assert.Nil(t, cfunc)
	})

	t.Run("Invalid TLS customization - Invalid max version given", func(t *testing.T) {
		cfunc, err := getTLSConfigCustomizer("1.2", "2.0", DefaultTLSCipherSuite)
		assert.Error(t, err)
		assert.Nil(t, cfunc)
	})

	t.Run("Invalid TLS customization - Unknown cipher suite given", func(t *testing.T) {
		cfunc, err := getTLSConfigCustomizer("1.3", "1.2", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:invalid")
		assert.Error(t, err)
		assert.Nil(t, cfunc)
	})

}

func TestBestEffortSystemCertPool(t *testing.T) {
	pool := BestEffortSystemCertPool()
	assert.NotNil(t, pool)
}
