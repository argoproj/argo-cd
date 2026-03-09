package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSelfSignedCerts(t *testing.T) {
	oldHealthCheckCert := healthCheckCert
	healthCheckCert = nil
	defer func() {
		healthCheckCert = oldHealthCheckCert
	}()

	// The first call should generate a new certificate
	config1, err := generateSelfSignedCerts()
	require.NoError(t, err)
	assert.False(t, config1.StrictValidation)
	require.Len(t, config1.ClientCertificates, 1)
	cert1 := config1.ClientCertificates[0]
	assert.NotNil(t, cert1.Leaf)

	// The second call should return the same cached certificate
	config2, err := generateSelfSignedCerts()
	require.NoError(t, err)
	require.Len(t, config2.ClientCertificates, 1)
	cert2 := config2.ClientCertificates[0]

	// Verify it's the same certificate (comparing the raw certificate data)
	assert.Equal(t, cert1.Certificate[0], cert2.Certificate[0])
}

func TestGenerateSelfSignedCerts_ThreadSafe(t *testing.T) {
	// Reset the global variables
	oldHealthCheckCert := healthCheckCert
	healthCheckCert = nil
	defer func() {
		healthCheckCert = oldHealthCheckCert
	}()

	const goroutines = 10
	results := make(chan struct {
		cert []byte
		err  error
	}, goroutines)

	for range goroutines {
		go func() {
			config, err := generateSelfSignedCerts()
			var cert []byte
			if err == nil && len(config.ClientCertificates) > 0 {
				cert = config.ClientCertificates[0].Certificate[0]
			}
			results <- struct {
				cert []byte
				err  error
			}{cert, err}
		}()
	}

	var firstCert []byte
	for range goroutines {
		res := <-results
		require.NoError(t, res.err)
		require.NotNil(t, res.cert)
		if firstCert == nil {
			firstCert = res.cert
		} else {
			assert.Equal(t, firstCert, res.cert)
		}
	}
}
