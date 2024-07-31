package profile

import (
	"net/http"
	"net/http/pprof"
	"os"
)

// RegisterProfiler adds pprof endpoints to mux.
func RegisterProfiler(mux *http.ServeMux) {
	addr := os.Getenv("ARGO_PPROF")
	if addr == "false" {
		return
	}

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}
