package swagger

import (
	"fmt"
	"log"
	"net/http"
	"path"

	"github.com/go-openapi/runtime/middleware"
	"github.com/gobuffalo/packr"
)

// ServeSwaggerUI serves the Swagger UI and JSON spec.
func ServeSwaggerUI(mux *http.ServeMux, component, uiPath string) {
	prefix := path.Dir(uiPath)
	specURL := path.Join(prefix, "swagger.json")

	box := packr.NewBox(path.Join("..", "..", component))
	swaggerJSON, err := box.MustString("swagger.json")
	if err != nil {
		log.Fatal(err)
	}

	mux.HandleFunc(specURL, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, swaggerJSON)
	})

	mux.Handle(uiPath, middleware.Redoc(middleware.RedocOpts{
		BasePath: prefix,
		SpecURL:  specURL,
		Path:     path.Base(uiPath),
	}, http.NotFoundHandler()))
}
