//go:build go1.18
// +build go1.18

// Copyright 2026 The Argo CD Authors
// SPDX-License-Identifier: Apache-2.0

package argocd_test

import (
	"testing"

	certutil "github.com/argoproj/argo-cd/v3/util/cert"
)

// FuzzDecodePEMCertificate tests PEM certificate parsing with
// arbitrary attacker-controlled PEM data.
//
// Argo CD parses TLS certificates from Git repositories,
// Helm charts, and user-provided configuration. Malformed
// certificate data must not cause a panic.
//
// 30 GitHub Security Advisories exist for Argo CD.
func FuzzDecodePEMCertificate(f *testing.F) {
	f.Add(`-----BEGIN CERTIFICATE-----
MIIDBjCCAe6gAwIBAgIQbJxYXbLmTnZQpEMNBCgFfDANBgkqhkiG9w0BAQsFADAW
MRQwEgYDVQQDEwtUZXN0IENBIENFUlQwHhcNMjQwMTAxMDAwMDAwWhcNMjUxMjMx
MjM1OTU5WjAWMRQwEgYDVQQDEwtUZXN0IENBIENFUlQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQC8fOcSJbSChqtpJcRtL+M8YK8QGqrSqfcHUOE0gA3A
HpIGzBS0sJBrSDI8jFkQeKzcqIBq7QXtTBY0B5PmS7nXy1pXjTiY0pMRqLCw0L7B
-----END CERTIFICATE-----`)
	f.Add("")
	f.Add("not-a-certificate")
	f.Add("-----BEGIN CERTIFICATE-----\ninvalid\n-----END CERTIFICATE-----")

	f.Fuzz(func(t *testing.T, pemData string) {
		if len(pemData) > 1<<16 {
			return
		}
		// Must never panic
		_, _ = certutil.DecodePEMCertificateToX509(pemData)
	})
}

// FuzzParseTLSCertificates tests TLS certificate data parsing
// with arbitrary multi-cert PEM bundles.
func FuzzParseTLSCertificates(f *testing.F) {
	f.Add("")
	f.Add("not-tls-data")
	f.Add("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----")

	f.Fuzz(func(t *testing.T, data string) {
		if len(data) > 1<<16 {
			return
		}
		_, _ = certutil.ParseTLSCertificatesFromData(data)
	})
}
