package swagger

import (
	"fmt"
	"net/http"
	"path"

	"github.com/go-openapi/runtime/server-middleware/docui"
)

// UI assets path where webpack copies the swagger-ui-dist files
const swaggerUIAssetsPath = "assets/swagger-ui"

// withFrameOptions wraps an http.Handler to set headers that prevent iframe embedding (clickjacking protection).
func withFrameOptions(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		h.ServeHTTP(w, r)
	})
}

// ServeSwaggerUI serves the Swagger UI and JSON spec.
func ServeSwaggerUI(mux *http.ServeMux, swaggerJSON string, uiPath string, rootPath string) {
	prefix := path.Dir(uiPath)
	swaggerPath := path.Join(prefix, "swagger.json")
	mux.Handle(swaggerPath, withFrameOptions(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, swaggerJSON)
	})))

	specURL := path.Join(prefix, rootPath, "swagger.json")
	// assets are served from the UI's own dist rather than docui's CDN defaults, so air-gapped installs work
	asset := func(name string) string {
		return path.Join(prefix, rootPath, swaggerUIAssetsPath, name)
	}
	mux.Handle(uiPath, withFrameOptions(docui.SwaggerUI(
		http.NotFoundHandler(),
		docui.WithUIBasePath(prefix),
		docui.WithSpecURL(specURL),
		docui.WithUIPath(path.Base(uiPath)),
		docui.WithUIAssetsURL(asset("swagger-ui-bundle.js")),
		docui.WithSwaggerUIOptions(docui.SwaggerUIOptions{
			SwaggerPresetURL: asset("swagger-ui-standalone-preset.js"),
			SwaggerStylesURL: asset("swagger-ui.css"),
			Favicon16:        asset("favicon-16x16.png"),
			Favicon32:        asset("favicon-32x32.png"),
		}),
	)))
}
