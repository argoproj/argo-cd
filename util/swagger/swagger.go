package swagger

import (
	"fmt"
	"net/http"
	"path"

	"github.com/go-openapi/runtime/middleware"
)

// filename of ReDoc script in UI's assets/scripts path
const redocScriptName = "redoc.standalone.js"

// setSecurityHeaders is a middleware that sets security headers on responses.
func setSecurityHeaders(xFrameOptions, contentSecurityPolicy string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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
}

// ServeSwaggerUI serves the Swagger UI and JSON spec with security headers.
func ServeSwaggerUI(mux *http.ServeMux, swaggerJSON string, uiPath string, rootPath string, xFrameOptions string, contentSecurityPolicy string) {
	prefix := path.Dir(uiPath)
	swaggerPath := path.Join(prefix, "swagger.json")

	swaggerHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", xFrameOptions)
		}
		if contentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		}
		_, _ = fmt.Fprint(w, swaggerJSON)
	})
	mux.Handle(swaggerPath, swaggerHandler)

	specURL := path.Join(prefix, rootPath, "swagger.json")
	scriptURL := path.Join(prefix, rootPath, "assets", "scripts", redocScriptName)
	redocHandler := middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
		RedocURL: scriptURL,
	}, http.NotFoundHandler())

	wrappedRedocHandler := setSecurityHeaders(xFrameOptions, contentSecurityPolicy)(redocHandler)
	mux.Handle(uiPath, wrappedRedocHandler)
}
