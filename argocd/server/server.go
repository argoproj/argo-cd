package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	argocd "github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/argocd/version"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	log "github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
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

// Run runs the API Server
func (a *ArgoCDServer) Run() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	// Create a cmux.
	m := cmux.New(conn)

	// Match connections in order: First gRPC, then HTTP
	grpcL := m.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
	httpL := m.Match(cmux.HTTP1Fast())

	// gRPC Server
	grpcS := grpc.NewServer()
	version.RegisterVersionServiceServer(grpcS, &version.Server{})

	// HTTP 1.1+JSON Server
	mux := http.NewServeMux()
	gwmux := runtime.NewServeMux()
	mux.Handle("/", gwmux)
	err = version.RegisterVersionServiceHandlerFromEndpoint(ctx, gwmux, endpoint, []grpc.DialOption{grpc.WithInsecure()})
	if err != nil {
		panic(err)
	}
	httpS := &http.Server{
		Addr:    endpoint,
		Handler: mux,
	}

	log.Infof("argocd %s serving on port %d", argocd.GetVersion(), port)

	// Use the muxed listeners for your servers.
	go grpcS.Serve(grpcL)
	go httpS.Serve(httpL)

	err = m.Serve()
	if err != nil {
		panic(err)
	}
}
