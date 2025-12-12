package swagger

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/assets"
)

func TestSwaggerUI(t *testing.T) {
	lc := &net.ListenConfig{}
	serve := func(c chan<- string) {
		// listen on first available dynamic (unprivileged) port
		listener, err := lc.Listen(t.Context(), "tcp", ":0")
		if err != nil {
			panic(err)
		}

		// send back the address so that it can be used
		c <- listener.Addr().String()

		mux := http.NewServeMux()
		ServeSwaggerUI(mux, assets.SwaggerJSON, "/swagger-ui", "", "", "")
		panic(http.Serve(listener, mux))
	}

	c := make(chan string, 1)

	// run a local webserver to test data retrieval
	go serve(c)

	address := <-c
	t.Logf("Listening at address: %s", address)

	server := "http://" + address

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

func TestSwaggerUISecurityHeaders(t *testing.T) {
	lc := &net.ListenConfig{}
	serve := func(c chan<- string) {
		listener, err := lc.Listen(t.Context(), "tcp", ":0")
		if err != nil {
			panic(err)
		}

		c <- listener.Addr().String()

		mux := http.NewServeMux()
		// Set security headers
		ServeSwaggerUI(mux, assets.SwaggerJSON, "/swagger-ui", "", "DENY", "frame-ancestors 'none';")
		panic(http.Serve(listener, mux))
	}

	c := make(chan string, 1)
	go serve(c)

	address := <-c
	t.Logf("Listening at address: %s", address)

	server := "http://" + address

	// Test swagger.json endpoint has security headers
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server+"/swagger.json", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"), "X-Frame-Options header should be set on swagger.json")
	require.Equal(t, "frame-ancestors 'none';", resp.Header.Get("Content-Security-Policy"), "Content-Security-Policy header should be set on swagger.json")
	require.NoError(t, resp.Body.Close())

	// Test swagger-ui endpoint has security headers
	req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, server+"/swagger-ui", http.NoBody)
	require.NoError(t, err)

	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"), "X-Frame-Options header should be set on swagger-ui")
	require.Equal(t, "frame-ancestors 'none';", resp.Header.Get("Content-Security-Policy"), "Content-Security-Policy header should be set on swagger-ui")
	require.NoError(t, resp.Body.Close())
}
