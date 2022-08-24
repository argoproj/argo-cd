package health

// Most simple health check probe to see whether our server is still alive

import (
	"fmt"
	"net/http"

	"github.com/argoproj/argo-cd/v2/image-updater/log"
)

func StartHealthServer(port int) chan error {
	errCh := make(chan error)
	go func() {
		sm := http.NewServeMux()
		sm.HandleFunc("/healthz", HealthProbe)
		errCh <- http.ListenAndServe(fmt.Sprintf(":%d", port), sm)
	}()
	return errCh
}

func HealthProbe(w http.ResponseWriter, r *http.Request) {
	log.Tracef("/healthz ping request received, replying with pong")
	fmt.Fprintf(w, "OK\n")
}
