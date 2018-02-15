package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/argoproj/argo"

	"github.com/argoproj/argo-cd/argocd/version"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	port = 8080
)

var (
	endpoint = fmt.Sprintf("localhost:%d", port)
)

// ArgoCDServer is the API server for ArgoCD
type ArgoCDServer struct {
}

// NewServer returns a new instance of the ArgoCD API server
func NewServer() *ArgoCDServer {
	return &ArgoCDServer{}
}

// grpcHandlerFunc returns an http.Handler that delegates to grpcServer on incoming gRPC
// connections or otherHandler otherwise. Copied from cockroachdb.
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(tamird): point to merged gRPC code rather than a PR.
		// This is a partial recreation of gRPC's internal checks https://github.com/grpc/grpc-go/pull/514/files#diff-95e9a25b738459a2d3030e1e6fa2a718R61
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}

// Run runs the API Server
func (a *ArgoCDServer) Run() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// GRPC server
	grpcServer := grpc.NewServer()
	opts := []grpc.DialOption{grpc.WithInsecure()}

	// HTTP 1.1+JSON Server
	mux := http.NewServeMux()
	gwmux := runtime.NewServeMux()
	mux.Handle("/", gwmux)

	// Register
	version.RegisterVersionServiceServer(grpcServer, &version.Server{})
	err := version.RegisterVersionServiceHandlerFromEndpoint(ctx, gwmux, endpoint, opts)
	if err != nil {
		panic(err)
	}

	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	srv := &http.Server{
		Addr:    endpoint,
		Handler: grpcHandlerFunc(grpcServer, mux),
	}
	log.Infof("argocd %s serving on port %d", argo.GetVersion(), port)
	err = srv.Serve(tls.NewListener(conn, srv.TLSConfig))
	if err != nil {
		panic(err)
	}
}
