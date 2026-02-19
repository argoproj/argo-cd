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
// xFrameOptions and contentSecurityPolicy are applied to all swagger responses
// when non-empty, consistent with how the static assets handler treats them.
func ServeSwaggerUI(mux *http.ServeMux, swaggerJSON string, uiPath string, rootPath string, xFrameOptions string, contentSecurityPolicy string) {
	setSecurityHeaders := func(w http.ResponseWriter) {
		if xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", xFrameOptions)
		}
		if contentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		}
	}

	prefix := path.Dir(uiPath)
	swaggerPath := path.Join(prefix, "swagger.json")
	mux.HandleFunc(swaggerPath, func(w http.ResponseWriter, _ *http.Request) {
		setSecurityHeaders(w)
		_, _ = fmt.Fprint(w, swaggerJSON)
	})

	specURL := path.Join(prefix, rootPath, "swagger.json")
	scriptURL := path.Join(prefix, rootPath, "assets", "scripts", redocScriptName)
	mux.Handle(uiPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		middleware.Redoc(middleware.RedocOpts{
			BasePath: prefix,
			SpecURL:  specURL,
			Path:     path.Base(uiPath),
			RedocURL: scriptURL,
		}, http.NotFoundHandler()).ServeHTTP(w, r)
	}))
}
