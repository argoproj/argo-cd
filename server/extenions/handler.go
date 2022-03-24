package extenions

import (
	"encoding/json"
	"net/http"
)

type Extension struct {
	Name string `json:"name"`
}

func NewHandler() http.Handler {
	extensions := []Extension{{"hello"}}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(extensions)
	})
}
