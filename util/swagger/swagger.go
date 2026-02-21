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
// xFrameOptions and contentSecurityPolicy are applied to all Swagger UI responses to
// prevent clickjacking attacks (see https://owasp.org/www-community/attacks/Clickjacking).
// Pass empty strings to omit the corresponding header.
func ServeSwaggerUI(mux *http.ServeMux, swaggerJSON string, uiPath string, rootPath string, xFrameOptions string, contentSecurityPolicy string) {
	prefix := path.Dir(uiPath)
	swaggerPath := path.Join(prefix, "swagger.json")
	mux.HandleFunc(swaggerPath, func(w http.ResponseWriter, _ *http.Request) {
		setSecurityHeaders(w, xFrameOptions, contentSecurityPolicy)
		_, _ = fmt.Fprint(w, swaggerJSON)
	})

	specURL := path.Join(prefix, rootPath, "swagger.json")
	scriptURL := path.Join(prefix, rootPath, "assets", "scripts", redocScriptName)
	uiHandler := middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
		RedocURL: scriptURL,
	}, http.NotFoundHandler())
	mux.Handle(uiPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w, xFrameOptions, contentSecurityPolicy)
		uiHandler.ServeHTTP(w, r)
	}))
}

// setSecurityHeaders adds X-Frame-Options and Content-Security-Policy headers when non-empty.
func setSecurityHeaders(w http.ResponseWriter, xFrameOptions string, contentSecurityPolicy string) {
	if xFrameOptions != "" {
		w.Header().Set("X-Frame-Options", xFrameOptions)
	}
	if contentSecurityPolicy != "" {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	}
}
