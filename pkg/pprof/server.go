package pprof

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	listener net.Listener
)

func init() {
	addr, exist := os.LookupEnv("ARGO_PPROF")
	if !exist {
		return
	}
	listener, _ = net.Listen("tcp", fmt.Sprintf(":%s", addr))
}

func IsEnabled() bool {
	return listener != nil
}

type pprofServer struct {
	server *http.Server
}

func NewPprofServer() (*pprofServer, error) {
	if listener == nil {
		return nil, fmt.Errorf("pprof server is disabled")
	}
	mux := http.NewServeMux()
	srv := &http.Server{
		Handler:           mux,
		MaxHeaderBytes:    1 << 20,
		IdleTimeout:       90 * time.Second, // matches http.DefaultTransport keep-alive timeout
		ReadHeaderTimeout: 32 * time.Second,
	}

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return &pprofServer{
		server: srv,
	}, nil
}

func (s *pprofServer) Start(ctx context.Context) error {
	if listener == nil {
		return fmt.Errorf("pprof server is disabled")
	}
	serverShutdown := make(chan struct{})
	go func() {
		<-ctx.Done()
		log.Info("shutting down server")
		if err := s.server.Shutdown(context.Background()); err != nil {
			log.Error(err, "error shutting down server")
		}
		close(serverShutdown)
	}()

	log.Info("Starting pprof server")
	if err := s.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	<-serverShutdown
	return nil
}

func (s *pprofServer) NeedLeaderElection() bool {
	return false
}
