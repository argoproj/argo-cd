package swagger

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/go-openapi/loads"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/assets"
)

func TestSwaggerUI(t *testing.T) {
	t.Parallel()

	// rootPath is the prefix the UI is served under, so it must appear in every generated URL
	for _, rootPath := range []string{"", "/argocd"} {
		t.Run(fmt.Sprintf("rootPath=%q", rootPath), func(t *testing.T) {
			t.Parallel()
			testSwaggerUI(t, rootPath)
		})
	}
}

func testSwaggerUI(t *testing.T, rootPath string) {
	t.Helper()

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
		ServeSwaggerUI(mux, assets.SwaggerJSON, "/swagger-ui", rootPath)
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

	// Verify clickjacking protection headers on swagger.json
	require.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	require.Equal(t, "frame-ancestors 'none'", resp.Header.Get("Content-Security-Policy"))

	// Verify clickjacking protection headers on swagger-ui
	uiReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server+"/swagger-ui", http.NoBody)
	require.NoError(t, err)

	uiResp, err := http.DefaultClient.Do(uiReq)
	require.NoError(t, err)
	require.Equalf(t, http.StatusOK, uiResp.StatusCode, "Was expecting status code 200 from swagger-ui, but got %d instead", uiResp.StatusCode)
	require.Equal(t, "DENY", uiResp.Header.Get("X-Frame-Options"))
	require.Equal(t, "frame-ancestors 'none'", uiResp.Header.Get("Content-Security-Policy"))

	uiBody, err := io.ReadAll(uiResp.Body)
	require.NoError(t, err)
	require.NoError(t, uiResp.Body.Close())

	// assets must be referenced from the UI's own dist, never docui's CDN defaults, so air-gapped installs work
	for _, asset := range []string{
		"swagger-ui-bundle.js",
		"swagger-ui-standalone-preset.js",
		"swagger-ui.css",
		"favicon-16x16.png",
		"favicon-32x32.png",
	} {
		require.Contains(t, string(uiBody), rootPath+"/assets/swagger-ui/"+asset)
	}
	require.NotContains(t, string(uiBody), "unpkg.com")
	require.NotContains(t, string(uiBody), "cdn.")

	// docui escapes the spec URL for its JS string context, e.g. \/argocd\/swagger.json
	require.Contains(t, strings.ReplaceAll(string(uiBody), `\/`, "/"), "url: '"+rootPath+"/swagger.json'")
}
