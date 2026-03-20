package swagger

import (
	"fmt"
	"net/http"
	"path"

	"github.com/go-openapi/runtime/middleware"
)

// filename of ReDoc script in UI's assets/scripts path
const redocScriptName = "redoc.standalone.js"

// ServeSwaggerUI serves the Swagger UI and JSON spec.
func ServeSwaggerUI(mux *http.ServeMux, swaggerJSON string, uiPath string, rootPath string, xFrameOptions string, contentSecurityPolicy string) {
	prefix := path.Dir(uiPath)
	swaggerPath := path.Join(prefix, "swagger.json")
	mux.HandleFunc(swaggerPath, withSecurityHeaders(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, swaggerJSON)
	}, xFrameOptions, contentSecurityPolicy))

	specURL := path.Join(prefix, rootPath, "swagger.json")
	scriptURL := path.Join(prefix, rootPath, "assets", "scripts", redocScriptName)
	mux.Handle(uiPath, withSecurityHeadersHandler(middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
		RedocURL: scriptURL,
	}, http.NotFoundHandler()), xFrameOptions, contentSecurityPolicy))
}

// withSecurityHeaders wraps an http.HandlerFunc with X-Frame-Options and Content-Security-Policy headers.
func withSecurityHeaders(next http.HandlerFunc, xFrameOptions string, contentSecurityPolicy string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", xFrameOptions)
		}
		if contentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		}
		next(w, r)
	}
}

// withSecurityHeadersHandler wraps an http.Handler with X-Frame-Options and Content-Security-Policy headers.
func withSecurityHeadersHandler(next http.Handler, xFrameOptions string, contentSecurityPolicy string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", xFrameOptions)
		}
		if contentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		}
		next.ServeHTTP(w, r)
	})
}
