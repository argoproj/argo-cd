package pull_request

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitLabTLSFixture(t *testing.T) (tls.Certificate, tls.Certificate, []byte) {
	t.Helper()
	path := "../../../test/fixture/certs/"
	serverCert, err := tls.LoadX509KeyPair(path+"argocd-test-server.crt", path+"argocd-test-server.key")
	require.NoError(t, err)
	clientCert, err := tls.LoadX509KeyPair(path+"argocd-test-client.crt", path+"argocd-test-client.key")
	require.NoError(t, err)
	ca, err := os.ReadFile(path + "argocd-test-ca.crt")
	require.NoError(t, err)
	return serverCert, clientCert, ca
}

func TestGitLabServiceMutualTLS(t *testing.T) {
	t.Parallel()
	serverCert, clientCert, ca := gitLabTLSFixture(t)
	for _, tc := range []struct {
		name    string
		cert    *tls.Certificate
		ca      []byte
		wantErr bool
	}{
		{name: "trusted client and server", cert: &clientCert, ca: ca},
		{name: "missing client certificate", ca: ca, wantErr: true},
		{name: "wrong client certificate usage", cert: &serverCert, ca: ca, wantErr: true},
		{name: "untrusted server", cert: &clientCert, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pool := x509.NewCertPool()
			require.True(t, pool.AppendCertsFromPEM(ca))
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "token-123", r.Header.Get("Private-Token"))
				assert.Equal(t, "/api/v4/projects/278964/merge_requests", r.URL.Path)
				require.NotEmpty(t, r.TLS.VerifiedChains)
				assert.Equal(t, "ArgoCD Test Client", r.TLS.PeerCertificates[0].Subject.CommonName)
				w.Header().Set("Content-Type", "application/json")
				writeMRListResponse(t, w)
			}))
			server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}
			server.StartTLS()
			defer server.Close()
			svc, err := NewGitLabService("token-123", server.URL, "278964", nil, nil, "opened", "", false, tc.ca, tc.cert, "", "")
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			prs, err := svc.List(ctx)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, prs, 1)
				assert.Equal(t, int64(15442), prs[0].Number)
			}
		})
	}
}

func TestGitLabServiceMutualTLSRequiresHTTPS(t *testing.T) {
	t.Parallel()
	_, cert, _ := gitLabTLSFixture(t)
	for _, url := range []string{"http://gitlab.example.com", "://invalid", "https:///missing-host"} {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			_, err := NewGitLabService("", url, "1", nil, nil, "", "", false, nil, &cert, "", "")
			require.ErrorContains(t, err, "must use HTTPS")
		})
	}
}

func TestGitLabServiceMutualTLSRedirects(t *testing.T) {
	t.Parallel()
	serverCert, clientCert, ca := gitLabTLSFixture(t)
	for _, crossOrigin := range []bool{false, true} {
		name := "same origin"
		if crossOrigin {
			name = "cross origin"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); writeMRListResponse(t, w) })
			target := httptest.NewUnstartedServer(handler)
			target.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequestClientCert}
			target.StartTLS()
			defer target.Close()
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirected" {
					handler.ServeHTTP(w, r)
					return
				}
				location := "/redirected"
				if crossOrigin {
					location = target.URL + "/redirected"
				}
				http.Redirect(w, r, location, http.StatusTemporaryRedirect)
			}))
			server.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}}
			server.StartTLS()
			defer server.Close()
			svc, err := NewGitLabService("", server.URL, "1", nil, nil, "", "", false, ca, &clientCert, "", "")
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v4/projects/1/merge_requests", http.NoBody)
			require.NoError(t, err)
			resp, err := svc.(*GitLabService).client.HTTPClient().Do(req)
			if resp != nil {
				defer resp.Body.Close()
			}
			if crossOrigin {
				require.ErrorContains(t, err, "cross-origin redirect")
				assert.Zero(t, calls.Load())
			} else {
				require.NoError(t, err)
				assert.Equal(t, int32(1), calls.Load())
			}
		})
	}
}
