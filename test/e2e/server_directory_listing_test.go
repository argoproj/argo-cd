package e2e

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// TestDirectoryListingDisabled verifies that the ArgoCD API server never returns
// an HTML directory listing for any directory path, and that the exposedFileSystem
// wrapper correctly blocks directory enumeration at the HTTP layer.
func TestDirectoryListingDisabled(t *testing.T) {
	testCases := []struct {
		name           string
		path           string
		acceptHeader   string
		expectedStatus int
		// bodyMustNotContain is checked when the status is 200/404 to make sure
		// the server never returns a raw directory listing.
		bodyMustNotContain []string
	}{
		{
			name:           "root with Accept:text/html serves SPA index",
			path:           "/",
			acceptHeader:   "text/html",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "root without Accept header serves embedded index.html",
			path:           "/",
			acceptHeader:   "",
			expectedStatus: http.StatusOK,
			// Should never be a raw directory listing.
			bodyMustNotContain: []string{"<pre>", "Index of"},
		},
		{
			name:           "assets directory without index.html returns 404",
			path:           "/assets/",
			acceptHeader:   "",
			expectedStatus: http.StatusNotFound,
			bodyMustNotContain: []string{
				"<pre>", "Index of",
				"<a href=\"favicon/\">favicon/</a>", "<a href=\"fonts/\">fonts/</a>", "<a href=\"images/\">images/</a>",
				"favicon", "fonts.css",
			},
		},
		{
			name:               "assets directory without trailing slash returns 404",
			path:               "/assets",
			acceptHeader:       "",
			expectedStatus:     http.StatusNotFound,
			bodyMustNotContain: []string{"<pre>", "Index of"},
		},
		{
			name:               "nested directory without index.html returns 404",
			path:               "/assets/favicon/",
			acceptHeader:       "",
			expectedStatus:     http.StatusNotFound,
			bodyMustNotContain: []string{"<pre>", "Index of", "favicon-16x16.png", "favicon-32x32.png"},
		},
		{
			name:           "static file inside directory is served normally",
			path:           "/assets/favicon/favicon.ico",
			acceptHeader:   "",
			expectedStatus: http.StatusOK,
		},
		{
			name:               "non-existent path returns 404 not directory listing",
			path:               "/this-path-does-not-exist/",
			acceptHeader:       "",
			expectedStatus:     http.StatusNotFound,
			bodyMustNotContain: []string{"<pre>", "Index of"},
		},
		{
			name:         "assets directory with Accept:text/html",
			path:         "/assets/",
			acceptHeader: "text/html",
			// when Accept:text/html, the server serves index.html for HTML clients
			expectedStatus:     http.StatusOK,
			bodyMustNotContain: []string{"<pre>", "<a href=\"images/\">images/</a>", "<a href=\"favicon/\">favicon/</a>"},
		},
	}

	Given(t).
		When().
		And(func() {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					var (
						resp *http.Response
						err  error
					)
					if tc.acceptHeader != "" {
						h := make(http.Header)
						h.Set("Accept", tc.acceptHeader)
						resp, err = DoHTTPRequestWithHeaders(http.MethodGet, tc.path, "", h)
					} else {
						resp, err = DoHttpRequest(http.MethodGet, tc.path, "")
					}
					require.NoError(t, err)
					defer func() {
						require.NoError(t, resp.Body.Close())
					}()

					assert.Equal(t, tc.expectedStatus, resp.StatusCode)

					if len(tc.bodyMustNotContain) > 0 {
						body, err := io.ReadAll(resp.Body)
						require.NoError(t, err)
						bodyStr := string(body)
						for _, forbidden := range tc.bodyMustNotContain {
							assert.NotContains(t, bodyStr, forbidden,
								"response body must not contain %q (directory listing must be disabled)", forbidden)
						}
					}
				})
			}
		})
}
