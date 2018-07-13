package swagger

import (
	"net/http"
	"path"
	"path/filepath"

	"github.com/go-openapi/runtime/middleware"
)

// ServeSwaggerUI serves the Swagger UI and JSON spec.
func ServeSwaggerUI(mux *http.ServeMux, component, uiPath string) {
	prefix := path.Dir(uiPath)
	specURL := path.Join(prefix, "swagger.json")

	mux.HandleFunc(specURL, func(w http.ResponseWriter, r *http.Request) {
		fp := filepath.Join(component, "swagger.json")
		http.ServeFile(w, r, fp)
	})

	mux.Handle(uiPath, middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
	}, http.NotFoundHandler()))
}
