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
// NOTE: This implementation adds X-Frame-Options header to prevent clickjacking attacks.
// See https://github.com/argoproj/argo-cd/issues/22877 for more details.
func ServeSwaggerUI(mux *http.ServeMux, swaggerJSON string, uiPath string, rootPath string) {
	prefix := path.Dir(uiPath)
	swaggerPath := path.Join(prefix, "swagger.json")

	// middleware to add security headers to all responses
	addSecurityHeaders := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Frame-Options", "DENY")
			next.ServeHTTP(w, r)
		})
	}

	// handler for swagger.json endpoint
	mux.HandleFunc(swaggerPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		_, _ = fmt.Fprint(w, swaggerJSON)
	})

	specURL := path.Join(prefix, rootPath, "swagger.json")
	scriptURL := path.Join(prefix, rootPath, "assets", "scripts", redocScriptName)

	// wrap Redoc handler with security headers middleware
	redocHandler := middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
		RedocURL: scriptURL,
	}, http.NotFoundHandler())

	// add security headers to swagger-ui endpoint
	mux.Handle(uiPath, addSecurityHeaders(redocHandler))

	// add security headers to all unmatched paths
	mux.Handle("/", addSecurityHeaders(http.NotFoundHandler()))
}
