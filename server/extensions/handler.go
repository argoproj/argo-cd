package extensions

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strings"
)

type Extension struct {
	URL string `json:"url"`
}

type Extensions map[string]Extension

type errorResponse struct {
	Message string `json:"message"`
}

func NewHandler() http.Handler {
	extensions := Extensions{
		"hello": {URL: "http://captive.apple.com"},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/extensions/"), "/")[0]

		log.WithField("name", name).Info("Extension")

		extension, ok := extensions[name]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(errorResponse{Message: fmt.Sprintf("extesionns %s not found", name)})
			return
		}

		resp, err := http.Get(extension.URL)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(errorResponse{Message: err.Error()})
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				w.WriteHeader(http.StatusBadGateway)
				_ = json.NewEncoder(w).Encode(errorResponse{Message: resp.Status})
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = io.Copy(w, resp.Body)
			}
		}
	})
}
