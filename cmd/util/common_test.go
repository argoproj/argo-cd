package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateBearerTokenAndPasswordCombo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		bearerToken string
		password    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Both token and password set",
			bearerToken: "some-token",
			password:    "some-password",
			expectError: true,
			errorMsg:    "only --bearer-token or --password is allowed, not both",
		},
		{
			name:        "Only token set",
			bearerToken: "some-token",
			password:    "",
			expectError: false,
		},
		{
			name:        "Only password set",
			bearerToken: "",
			password:    "some-password",
			expectError: false,
		},
		{
			name:        "Neither token nor password set",
			bearerToken: "",
			password:    "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateBearerTokenAndPasswordCombo(tt.bearerToken, tt.password)
			if tt.expectError {
				require.ErrorContains(t, err, tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateBearerTokenForGitOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		bearerToken string
		repoType    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Bearer token with helm repo",
			bearerToken: "some-token",
			repoType:    "helm",
			expectError: true,
			errorMsg:    "--bearer-token is only supported for Git repositories",
		},
		{
			name:        "Bearer token with git repo",
			bearerToken: "some-token",
			repoType:    "git",
			expectError: false,
		},
		{
			name:        "No bearer token with helm repo",
			bearerToken: "",
			repoType:    "helm",
			expectError: false,
		},
		{
			name:        "No bearer token with git repo",
			bearerToken: "",
			repoType:    "git",
			expectError: false,
		},
		{
			name:        "Bearer token with empty repo",
			bearerToken: "some-token",
			repoType:    "",
			expectError: true,
			errorMsg:    "--bearer-token is only supported for Git repositories",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateBearerTokenForGitOnly(tt.bearerToken, tt.repoType)
			if tt.expectError {
				require.ErrorContains(t, err, tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateBearerTokenForHTTPSRepoOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		bearerToken string
		isHTTPS     bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Bearer token with HTTPS repo",
			bearerToken: "some-token",
			isHTTPS:     true,
			expectError: false,
		},
		{
			name:        "Bearer token with non-HTTPS repo",
			bearerToken: "some-token",
			isHTTPS:     false,
			expectError: true,
			errorMsg:    "--bearer-token is only supported for HTTPS repositories",
		},
		{
			name:        "No bearer token with HTTPS repo",
			bearerToken: "",
			isHTTPS:     true,
			expectError: false,
		},
		{
			name:        "No bearer token with non-HTTPS repo",
			bearerToken: "",
			isHTTPS:     false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateBearerTokenForHTTPSRepoOnly(tt.bearerToken, tt.isHTTPS)
			if tt.expectError {
				require.ErrorContains(t, err, tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateInsecureOCIForceHTTP(t *testing.T) {
	tests := []struct {
		name                 string
		insecureOCIForceHTTP bool
		repoType             string
		enableOCI            bool
		expectError          bool
	}{
		{
			name:                 "type=oci with insecure-oci-force-http",
			insecureOCIForceHTTP: true,
			repoType:             "oci",
			enableOCI:            false,
			expectError:          false,
		},
		{
			name:                 "type=helm with enable-oci and insecure-oci-force-http",
			insecureOCIForceHTTP: true,
			repoType:             "helm",
			enableOCI:            true,
			expectError:          false,
		},
		{
			name:                 "type=helm without enable-oci and insecure-oci-force-http",
			insecureOCIForceHTTP: true,
			repoType:             "helm",
			enableOCI:            false,
			expectError:          true,
		},
		{
			name:                 "type=git with insecure-oci-force-http",
			insecureOCIForceHTTP: true,
			repoType:             "git",
			enableOCI:            false,
			expectError:          true,
		},
		{
			name:                 "type=git with enable-oci and insecure-oci-force-http",
			insecureOCIForceHTTP: true,
			repoType:             "git",
			enableOCI:            true,
			expectError:          true,
		},
		{
			name:                 "insecure-oci-force-http not set on git repo",
			insecureOCIForceHTTP: false,
			repoType:             "git",
			enableOCI:            false,
			expectError:          false,
		},
		{
			name:                 "insecure-oci-force-http not set on helm repo",
			insecureOCIForceHTTP: false,
			repoType:             "helm",
			enableOCI:            false,
			expectError:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInsecureOCIForceHTTP(tt.insecureOCIForceHTTP, tt.repoType, tt.enableOCI)
			if tt.expectError {
				require.ErrorContains(t, err, "--insecure-oci-force-http requires --type oci or --enable-oci")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
