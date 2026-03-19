package swagger

import (
	"fmt"
	"net/http"
	"path"

	"github.com/go-openapi/runtime/middleware"
)

// filename of ReDoc script in UI's assets/scripts path
const redocScriptName = "redoc.standalone.js"

// withSecurityHeaders wraps an http.Handler to add clickjacking-prevention headers.
func withSecurityHeaders(h http.Handler, xFrameOptions, contentSecurityPolicy string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", xFrameOptions)
		}
		if contentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		}
		h.ServeHTTP(w, r)
	})
}

// ServeSwaggerUI serves the Swagger UI and JSON spec.
func ServeSwaggerUI(mux *http.ServeMux, swaggerJSON string, uiPath string, rootPath string, xFrameOptions string, contentSecurityPolicy string) {
	prefix := path.Dir(uiPath)
	swaggerPath := path.Join(prefix, "swagger.json")
	mux.Handle(swaggerPath, withSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, swaggerJSON)
	}), xFrameOptions, contentSecurityPolicy))

	specURL := path.Join(prefix, rootPath, "swagger.json")
	scriptURL := path.Join(prefix, rootPath, "assets", "scripts", redocScriptName)
	mux.Handle(uiPath, withSecurityHeaders(middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
		RedocURL: scriptURL,
	}, http.NotFoundHandler()), xFrameOptions, contentSecurityPolicy))
}
