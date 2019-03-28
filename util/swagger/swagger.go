package swagger

import (
	"fmt"
	"net/http"
	"path"

	"github.com/go-openapi/runtime/middleware"
)

// ServeSwaggerUI serves the Swagger UI and JSON spec.
func ServeSwaggerUI(mux *http.ServeMux, swaggerJSON string, uiPath string) {
	prefix := path.Dir(uiPath)
	specURL := path.Join(prefix, "swagger.json")

	mux.HandleFunc(specURL, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, swaggerJSON)
	})

	mux.Handle(uiPath, middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
	}, http.NotFoundHandler()))
}
