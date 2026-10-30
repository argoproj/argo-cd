package pull_request

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeMRListResponse(t *testing.T, w io.Writer) {
	t.Helper()
	f, err := os.Open("fixtures/gitlab_mr_list_response.json")
	require.NoErrorf(t, err, "error opening fixture file: %v", err)

	_, err = io.Copy(w, f)
	require.NoErrorf(t, err, "error writing response: %v", err)
}

func TestGitLabServiceCustomBaseURL(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/278964/merge_requests"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path+"?per_page=100", r.URL.RequestURI())
		writeMRListResponse(t, w)
	})

	svc, err := NewGitLabService("", server.URL, "278964", nil, nil, "", "", false, nil, nil, "", "")
	require.NoError(t, err)

	_, err = svc.List(t.Context())
	require.NoError(t, err)
}

func TestGitLabServiceToken(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/278964/merge_requests"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token-123", r.Header.Get("Private-Token"))
		writeMRListResponse(t, w)
	})

	svc, err := NewGitLabService("token-123", server.URL, "278964", nil, nil, "", "", false, nil, nil, "", "")
	require.NoError(t, err)

	_, err = svc.List(t.Context())
	require.NoError(t, err)
}

func TestList(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/278964/merge_requests"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path+"?per_page=100", r.URL.RequestURI())
		writeMRListResponse(t, w)
	})

	svc, err := NewGitLabService("", server.URL, "278964", []string{}, nil, "", "", false, nil, nil, "", "")
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)
	assert.Len(t, prs, 1)
	assert.Equal(t, int64(15442), prs[0].Number)
	assert.Equal(t, "Draft: Use structured logging for DB load balancer", prs[0].Title)
	assert.Equal(t, "use-structured-logging-for-db-load-balancer", prs[0].Branch)
	assert.Equal(t, "master", prs[0].TargetBranch)
	assert.Equal(t, "2fc4e8b972ff3208ec63b6143e34ad67ff343ad7", prs[0].HeadSHA)
	assert.Equal(t, "hfyngvason", prs[0].Author)
}

func TestListWithLabels(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/278964/merge_requests"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path+"?labels=feature%2Cready&per_page=100", r.URL.RequestURI())
		writeMRListResponse(t, w)
	})

	svc, err := NewGitLabService("", server.URL, "278964", []string{"feature", "ready"}, nil, "", "", false, nil, nil, "", "")
	require.NoError(t, err)

	_, err = svc.List(t.Context())
	require.NoError(t, err)
}

func TestListWithState(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/278964/merge_requests"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path+"?per_page=100&state=opened", r.URL.RequestURI())
		writeMRListResponse(t, w)
	})

	svc, err := NewGitLabService("", server.URL, "278964", []string{}, nil, "opened", "", false, nil, nil, "", "")
	require.NoError(t, err)

	_, err = svc.List(t.Context())
	require.NoError(t, err)
}

// TestListWithStateTLS checks that state filtering preserves the configured server certificate verification behavior.
func TestListWithStateTLS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		tlsInsecure bool
		passCerts   bool
		requireErr  bool
	}{
		{
			name:        "TLS Insecure: true, No Certs",
			tlsInsecure: true,
			passCerts:   false,
			requireErr:  false,
		},
		{
			name:        "TLS Insecure: true, With Certs",
			tlsInsecure: true,
			passCerts:   true,
			requireErr:  false,
		},
		{
			name:        "TLS Insecure: false, With Certs",
			tlsInsecure: false,
			passCerts:   true,
			requireErr:  false,
		},
		{
			name:        "TLS Insecure: false, No Certs",
			tlsInsecure: false,
			passCerts:   false,
			requireErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeMRListResponse(t, w)
			}))
			defer ts.Close()

			var certs []byte
			if test.passCerts {
				for _, cert := range ts.TLS.Certificates {
					for _, c := range cert.Certificate {
						parsedCert, err := x509.ParseCertificate(c)
						require.NoError(t, err, "Failed to parse certificate")
						certs = append(certs, pem.EncodeToMemory(&pem.Block{
							Type:  "CERTIFICATE",
							Bytes: parsedCert.Raw,
						})...)
					}
				}
			}

			svc, err := NewGitLabService("", ts.URL, "278964", []string{}, nil, "opened", "", test.tlsInsecure, certs, nil, "", "")
			require.NoError(t, err)

			_, err = svc.List(t.Context())
			if test.requireErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGitLabListReturnsRepositoryNotFoundError(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/nonexistent/merge_requests"

	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		// Return 404 status to simulate repository not found
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "404 Project Not Found"}`))
	})

	svc, err := NewGitLabService("", server.URL, "nonexistent", []string{}, nil, "", "", false, nil, nil, "", "")
	require.NoError(t, err)

	prs, err := svc.List(t.Context())

	// Should return empty pull requests list
	assert.Empty(t, prs)

	// Should return RepositoryNotFoundError
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}

func TestListWithExcludedLabels(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/278964/merge_requests"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path+"?per_page=100", r.URL.RequestURI())
		writeMRListResponse(t, w)
	})

	// The single MR in the fixture carries the "backend" label, so excluding it yields no MRs.
	svc, err := NewGitLabService("", server.URL, "278964", nil, []string{"backend"}, "", "", false, nil, nil, "", "")
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)
	assert.Empty(t, prs)
}

// gitLabTLSFixture loads the test CA and separate client/server identities used by GitLab transport tests.
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

// TestGitLabServiceMutualTLS checks verified client authentication and rejection of missing credentials, invalid usage, and untrusted servers.
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

// TestGitLabServiceMutualTLSRequiresHTTPS prevents using a client identity with a plaintext or malformed API endpoint.
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

// TestGitLabServiceMutualTLSRedirects permits same-origin redirects while keeping the client identity away from other origins.
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
