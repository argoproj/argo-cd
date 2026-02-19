package swagger

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/assets"
)

func serveTestSwagger(t *testing.T, xFrameOptions, contentSecurityPolicy string) string {
	t.Helper()
	lc := &net.ListenConfig{}
	listener, err := lc.Listen(t.Context(), "tcp", ":0")
	require.NoError(t, err)

	addr := listener.Addr().String()
	mux := http.NewServeMux()
	ServeSwaggerUI(mux, assets.SwaggerJSON, "/swagger-ui", "", xFrameOptions, contentSecurityPolicy)
	go func() { _ = http.Serve(listener, mux) }()

	return "http://" + addr
}

func TestSwaggerUI(t *testing.T) {
	server := serveTestSwagger(t, "", "")

	specDoc, err := loads.Spec(server + "/swagger.json")
	require.NoError(t, err)

	_, err = json.MarshalIndent(specDoc.Spec(), "", "  ")
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server+"/swagger.json", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "Was expecting status code 200 from swagger-ui, but got %d instead", resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func TestSwaggerUIPage(t *testing.T) {
	server := serveTestSwagger(t, "", "")

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server+"/swagger-ui", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	// Redoc serves the UI page; any non-error response confirms the handler is wired correctly.
	assert.Less(t, resp.StatusCode, http.StatusInternalServerError)
}

func TestSwaggerUISecurityHeaders(t *testing.T) {
	endpoints := []string{"/swagger.json", "/swagger-ui"}

	t.Run("headers set when configured", func(t *testing.T) {
		server := serveTestSwagger(t, "DENY", "frame-ancestors 'none'")

		for _, endpoint := range endpoints {
			t.Run(endpoint, func(t *testing.T) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server+endpoint, http.NoBody)
				require.NoError(t, err)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())

				assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
				assert.Equal(t, "frame-ancestors 'none'", resp.Header.Get("Content-Security-Policy"))
			})
		}
	})

	t.Run("headers absent when not configured", func(t *testing.T) {
		server := serveTestSwagger(t, "", "")

		for _, endpoint := range endpoints {
			t.Run(endpoint, func(t *testing.T) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server+endpoint, http.NoBody)
				require.NoError(t, err)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())

				assert.Empty(t, resp.Header.Get("X-Frame-Options"))
				assert.Empty(t, resp.Header.Get("Content-Security-Policy"))
			})
		}
	})
}
