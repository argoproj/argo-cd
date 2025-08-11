package generators

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestPhaseDeploymentProcessor_runHTTPCheck(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	appSet := &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: "default",
		},
	}

	tests := []struct {
		name         string
		server       func() *httptest.Server
		check        argoprojiov1alpha1.GeneratorPhaseCheck
		expectError  bool
		errorMessage string
	}{
		{
			name: "successful GET request",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "test-appset", r.Header.Get("X-AppSet-Name"))
					assert.Equal(t, "default", r.Header.Get("X-AppSet-Namespace"))
					assert.Equal(t, "health-check", r.Header.Get("X-Check-Name"))
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("OK"))
				}))
			},
			check: argoprojiov1alpha1.GeneratorPhaseCheck{
				Name: "health-check",
				Type: "http",
				HTTP: &argoprojiov1alpha1.GeneratorPhaseCheckHTTP{
					URL: "", // Will be set to server URL
				},
			},
			expectError: false,
		},
		{
			name: "successful POST request with body",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte("Created"))
				}))
			},
			check: argoprojiov1alpha1.GeneratorPhaseCheck{
				Name: "api-check",
				Type: "http",
				HTTP: &argoprojiov1alpha1.GeneratorPhaseCheckHTTP{
					URL:    "", // Will be set to server URL
					Method: "POST",
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body:           `{"test": "data"}`,
					ExpectedStatus: 201,
				},
			},
			expectError: false,
		},
		{
			name: "failed request - wrong status code",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal Server Error"))
				}))
			},
			check: argoprojiov1alpha1.GeneratorPhaseCheck{
				Name: "failing-check",
				Type: "http",
				HTTP: &argoprojiov1alpha1.GeneratorPhaseCheckHTTP{
					URL:            "", // Will be set to server URL
					ExpectedStatus: 200,
				},
			},
			expectError:  true,
			errorMessage: "expected status 200, got 500",
		},
		{
			name: "request with custom expected status",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte("Not Found"))
				}))
			},
			check: argoprojiov1alpha1.GeneratorPhaseCheck{
				Name: "custom-status-check",
				Type: "http",
				HTTP: &argoprojiov1alpha1.GeneratorPhaseCheckHTTP{
					URL:            "", // Will be set to server URL
					ExpectedStatus: 404,
				},
			},
			expectError: false,
		},
		{
			name: "request with multiple headers",
			server: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
					assert.Equal(t, "application/json", r.Header.Get("Accept"))
					assert.Equal(t, "ArgoCD-ApplicationSet-PhaseCheck/1.0", r.Header.Get("User-Agent"))
					w.WriteHeader(http.StatusOK)
				}))
			},
			check: argoprojiov1alpha1.GeneratorPhaseCheck{
				Name: "auth-check",
				Type: "http",
				HTTP: &argoprojiov1alpha1.GeneratorPhaseCheckHTTP{
					URL: "", // Will be set to server URL
					Headers: map[string]string{
						"Authorization": "Bearer token123",
						"Accept":        "application/json",
					},
				},
			},
			expectError: false,
		},
		{
			name:   "no HTTP config",
			server: func() *httptest.Server { return nil },
			check: argoprojiov1alpha1.GeneratorPhaseCheck{
				Name: "no-http-config",
				Type: "http",
			},
			expectError:  true,
			errorMessage: "http check requires http field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.server != nil {
				server = tt.server()
				if server != nil {
					defer server.Close()
					if tt.check.HTTP != nil {
						tt.check.HTTP.URL = server.URL
					}
				}
			}

			ctx := context.Background()
			err := processor.runHTTPCheck(ctx, appSet, tt.check)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPhaseDeploymentProcessor_runHTTPCheck_Timeout(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	// Create a server that takes longer than the timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Sleep longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	appSet := &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: "default",
		},
	}

	check := argoprojiov1alpha1.GeneratorPhaseCheck{
		Name: "timeout-check",
		Type: "http",
		HTTP: &argoprojiov1alpha1.GeneratorPhaseCheckHTTP{
			URL: server.URL,
		},
		Timeout: &metav1.Duration{Duration: 100 * time.Millisecond},
	}

	ctx := context.Background()
	logger := log.WithField("test", "runSingleCheck")
	err := processor.runSingleCheck(ctx, appSet, check, logger)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestPhaseDeploymentProcessor_runHTTPCheck_HTTPS(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	processor := NewPhaseDeploymentProcessor(client)

	// Create HTTPS test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("HTTPS OK"))
	}))
	defer server.Close()

	appSet := &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: "default",
		},
	}

	t.Run("HTTPS with insecure skip verify", func(t *testing.T) {
		check := argoprojiov1alpha1.GeneratorPhaseCheck{
			Name: "https-insecure-check",
			Type: "http",
			HTTP: &argoprojiov1alpha1.GeneratorPhaseCheckHTTP{
				URL:                server.URL,
				InsecureSkipVerify: true,
			},
		}

		ctx := context.Background()
		err := processor.runHTTPCheck(ctx, appSet, check)
		assert.NoError(t, err)
	})

	t.Run("HTTPS without insecure skip verify", func(t *testing.T) {
		check := argoprojiov1alpha1.GeneratorPhaseCheck{
			Name: "https-secure-check",
			Type: "http",
			HTTP: &argoprojiov1alpha1.GeneratorPhaseCheckHTTP{
				URL:                server.URL,
				InsecureSkipVerify: false,
			},
		}

		ctx := context.Background()
		err := processor.runHTTPCheck(ctx, appSet, check)
		// This should fail because we're using a self-signed certificate
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "certificate")
	})
}