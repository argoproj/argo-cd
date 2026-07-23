package apiclient

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateServerAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		server  string
		wantErr string
	}{
		{name: "hostname", server: "argocd.example.com"},
		{name: "hostname and port", server: "argocd.example.com:8443"},
		{name: "IPv4 address", server: "127.0.0.1:8080"},
		{name: "bracketed IPv6 address", server: "[::1]:8080"},
		{
			name:    "unsupported resolver scheme",
			server:  "dns:///argocd-server.argocd.svc.cluster.local:443",
			wantErr: `server address "dns:///argocd-server.argocd.svc.cluster.local:443" must be a host and optional port without a URL scheme`,
		},
		{
			name:    "HTTPS URL",
			server:  "https://argocd.example.com:8443",
			wantErr: `server address "https://argocd.example.com:8443" must not include a URL scheme; use "argocd.example.com:8443" instead`,
		},
		{
			name:    "HTTP URL",
			server:  "http://localhost:8080",
			wantErr: `server address "http://localhost:8080" must not include a URL scheme; use "localhost:8080" with --plaintext instead`,
		},
		{
			name:    "HTTPS URL with root path",
			server:  "https://argocd.example.com/argocd",
			wantErr: `server address "https://argocd.example.com/argocd" must not include a URL scheme or path; use "argocd.example.com" with --grpc-web-root-path "/argocd" instead`,
		},
		{
			name:    "HTTP URL with root path",
			server:  "http://localhost:8080/argocd/",
			wantErr: `server address "http://localhost:8080/argocd/" must not include a URL scheme or path; use "localhost:8080" with --plaintext --grpc-web-root-path "/argocd" instead`,
		},
		{
			name:    "host and root path",
			server:  "localhost:8080/argocd",
			wantErr: `server address "localhost:8080/argocd" must not include a path; use "localhost:8080" with --grpc-web-root-path "/argocd" instead`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateServerAddress(test.server)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, test.wantErr)
		})
	}
}

func TestNewClientRejectsHTTPURLServerAddress(t *testing.T) {
	_, err := NewClient(&ClientOptions{
		ConfigPath: filepath.Join(t.TempDir(), "config"),
		ServerAddr: "https://localhost:8080",
	})

	require.EqualError(t, err, `server address "https://localhost:8080" must not include a URL scheme; use "localhost:8080" instead`)
}
