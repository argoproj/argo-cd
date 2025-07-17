package profile

import (
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/argoproj/argo-cd/v2/util/env"
)

var enableProfilerFilePath = env.StringFromEnv("ARGOCD_ENABLE_PROFILER_FILE_PATH", "/home/argocd/params/profiler.enabled")

func wrapHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if data, err := os.ReadFile(enableProfilerFilePath); err == nil && string(data) == "true" {
			handler.ServeHTTP(w, r)
		} else {
			http.Error(w, "Profiler endpoint is not enabled in 'argocd-cmd-params-cm' ConfigMap", http.StatusUnauthorized)
		}
	}
}

// RegisterProfiler adds pprof endpoints to mux.
func RegisterProfiler(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", wrapHandler(pprof.Index))
	mux.HandleFunc("/debug/pprof/cmdline", wrapHandler(pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", wrapHandler(pprof.Profile))
	mux.HandleFunc("/debug/pprof/symbol", wrapHandler(pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", wrapHandler(pprof.Trace))
}
