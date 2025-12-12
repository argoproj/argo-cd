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
	mux.HandleFunc(swaggerPath, func(w http.ResponseWriter, _ *http.Request) {
		// Set security headers
		if xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", xFrameOptions)
		}
		if contentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		}
		_, _ = fmt.Fprint(w, swaggerJSON)
	})

	specURL := path.Join(prefix, rootPath, "swagger.json")
	scriptURL := path.Join(prefix, rootPath, "assets", "scripts", redocScriptName)

	// Wrap the Redoc handler to add security headers
	redocHandler := middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
		RedocURL: scriptURL,
	}, http.NotFoundHandler())

	mux.HandleFunc(uiPath, func(w http.ResponseWriter, r *http.Request) {
		// Set security headers
		if xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", xFrameOptions)
		}
		if contentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		}
		redocHandler.ServeHTTP(w, r)
	})
}
