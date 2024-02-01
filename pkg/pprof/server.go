package pprof

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	log "github.com/sirupsen/logrus"
)

type server struct {
	server   *http.Server
	listener *net.Listener
}

func NewPprofServer(addr string) (server, error) {
	if addr == "" || addr == "0" {
		return server{}, nil
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return server{}, fmt.Errorf("error listening on %s: %w", addr, err)
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
	return server{
		listener: &ln,
		server:   srv,
	}, nil
}

func (s *server) Start(ctx context.Context) error {
	if s.listener == nil {
		return nil
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
	if err := s.server.Serve(*s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	<-serverShutdown
	return nil
}

func (s *server) NeedLeaderElection() bool {
	return false
}
