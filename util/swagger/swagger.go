package swagger

import (
	"fmt"
	"net/http"
	"path"

	"github.com/go-openapi/runtime/middleware"
)

// filename of ReDoc script in UI's assets/scripts path
const redocScriptName = "redoc.standalone.js"

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
	mux.HandleFunc(swaggerPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		_, _ = fmt.Fprint(w, swaggerJSON)
	})

	specURL := path.Join(prefix, rootPath, "swagger.json")
	scriptURL := path.Join(prefix, rootPath, "assets", "scripts", redocScriptName)
	mux.Handle(uiPath, withFrameOptions(middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
		RedocURL: scriptURL,
	}, http.NotFoundHandler())))
}
