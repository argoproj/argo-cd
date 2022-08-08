package healthz

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// ServeHealthCheck serves the health check endpoint.
// ServeHealthCheck relies on the provided function to return an error if unhealthy and nil otherwise.
func ServeHealthCheck(mux *http.ServeMux, f func(r *http.Request) error) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := f(r); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			log.Errorln(w, err)
		} else {
			fmt.Fprintln(w, "ok")
		}
	})
}
