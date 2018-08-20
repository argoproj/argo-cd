package healthz

import (
	"fmt"
	"net/http"
)

// ServeHealthCheck serves the health check endpoint.
// ServeHealthCheck relies on the provided function to return an error if unavailable and nil otherwise.
func ServeHealthCheck(mux *http.ServeMux, f func() error) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := f(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, err)
		} else {
			fmt.Fprintln(w, "ok")
		}
	})
}
