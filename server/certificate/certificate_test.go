package certificate

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	certificatepkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/certificate"
	"github.com/argoproj/argo-cd/v3/util/assets"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	testNamespace = "default"

	// Public host key of github.com as published by GitHub
	testSSHKnownHosts = "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n" +
		"[ssh.github.com]:443 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n"
)

func newTestServer(t *testing.T, allowed bool) *Server {
	t.Helper()
	labels := map[string]string{"app.kubernetes.io/part-of": "argocd"}
	clientset := fake.NewClientset(
		&corev1.ConfigMap{Name: common.ArgoCDConfigMapName, Namespace: testNamespace, Labels: labels},
		&corev1.ConfigMap{
			Name:      common.ArgoCDKnownHostsConfigMapName,
			Namespace: testNamespace,
			Labels:    labels,
			Data:      map[string]string{"ssh_known_hosts": testSSHKnownHosts},
		},
		&corev1.ConfigMap{Name: common.ArgoCDTLSCertsConfigMapName, Namespace: testNamespace, Labels: labels},
	)
	settingsMgr := settings.NewSettingsManager(t.Context(), clientset, testNamespace)

	enforcer := rbac.NewEnforcer(clientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	require.NoError(t, enforcer.SetBuiltinPolicy(assets.BuiltinPolicyCSV))
	if allowed {
		enforcer.SetDefaultRole("role:admin")
	}
	enforcer.SetClaimsEnforcerFunc(func(_ jwt.Claims, _ ...any) bool {
		return allowed
	})

	return NewServer(db.NewDB(testNamespace, settingsMgr, clientset), enforcer)
}

func TestGetCertificate(t *testing.T) {
	t.Parallel()

	t.Run("returns the certificate data", func(t *testing.T) {
		t.Parallel()
		server := newTestServer(t, true)
		certList, err := server.GetCertificate(t.Context(), &certificatepkg.RepositoryCertificateGetRequest{ServerName: "github.com"})
		require.NoError(t, err)
		require.Len(t, certList.Items, 1)
		cert := certList.Items[0]
		assert.Equal(t, "github.com", cert.ServerName)
		assert.Equal(t, "ssh", cert.CertType)
		assert.Equal(t, "ssh-ed25519", cert.CertSubType)
		assert.Equal(t, "AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl", string(cert.CertData))
		assert.Equal(t, "SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU", cert.CertInfo)
	})

	t.Run("unknown server is not found", func(t *testing.T) {
		t.Parallel()
		server := newTestServer(t, true)
		_, err := server.GetCertificate(t.Context(), &certificatepkg.RepositoryCertificateGetRequest{ServerName: "unknown.example.com"})
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("empty server name is rejected", func(t *testing.T) {
		t.Parallel()
		server := newTestServer(t, true)
		_, err := server.GetCertificate(t.Context(), &certificatepkg.RepositoryCertificateGetRequest{})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("permission denied without get access", func(t *testing.T) {
		t.Parallel()
		server := newTestServer(t, false)
		_, err := server.GetCertificate(t.Context(), &certificatepkg.RepositoryCertificateGetRequest{ServerName: "github.com"})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
}

// Server names of SSH entries can contain a colon, which grpc-gateway would
// mistake for a custom verb if it ended up in the last path segment.
func TestGetCertificateHTTPRoute(t *testing.T) {
	t.Parallel()
	mux := runtime.NewServeMux()
	require.NoError(t, certificatepkg.RegisterCertificateServiceHandlerServer(t.Context(), mux, newTestServer(t, true)))

	tests := []struct {
		name       string
		path       string
		statusCode int
		serverName string
	}{
		{name: "plain server name", path: "/api/v1/certificates/github.com/details", statusCode: http.StatusOK, serverName: "github.com"},
		{name: "server name with port", path: "/api/v1/certificates/" + url.PathEscape("[ssh.github.com]:443") + "/details", statusCode: http.StatusOK, serverName: "[ssh.github.com]:443"},
		{name: "unknown server", path: "/api/v1/certificates/unknown.example.com/details", statusCode: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody))

			require.Equal(t, tt.statusCode, rec.Code, rec.Body.String())
			if tt.serverName != "" {
				assert.Contains(t, rec.Body.String(), `"serverName":"`+tt.serverName+`"`)
			}
		})
	}
}
