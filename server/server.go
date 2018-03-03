package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"strings"

	argocd "github.com/argoproj/argo-cd"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/server/version"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	jsonutil "github.com/argoproj/argo-cd/util/json"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	log "github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
)

const (
	port = 8080
)

var (
	endpoint = fmt.Sprintf("localhost:%d", port)
)

// ArgoCDServer is the API server for ArgoCD
type ArgoCDServer struct {
	ns              string
	staticAssetsDir string
	kubeclientset   kubernetes.Interface
	appclientset    appclientset.Interface
	log             *log.Entry
}

// NewServer returns a new instance of the ArgoCD API server
func NewServer(kubeclientset kubernetes.Interface, appclientset appclientset.Interface, namespace string, staticAssetsDir string) *ArgoCDServer {
	return &ArgoCDServer{
		ns:              namespace,
		kubeclientset:   kubeclientset,
		appclientset:    appclientset,
		log:             log.NewEntry(log.New()),
		staticAssetsDir: staticAssetsDir,
	}
}

// Run runs the API Server
// We use k8s.io/code-generator/cmd/go-to-protobuf to generate the .proto files from the API types.
// k8s.io/ go-to-protobuf uses protoc-gen-gogo, which comes from gogo/protobuf (a fork of
// golang/protobuf).
func (a *ArgoCDServer) Run() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	// Cmux is used to support servicing gRPC and HTTP1.1+JSON on the same port
	m := cmux.New(conn)
	grpcL := m.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
	httpL := m.Match(cmux.HTTP1Fast())

	// gRPC Server
	grpcS := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_logrus.StreamServerInterceptor(a.log),
			grpc_util.PanicLoggerStreamServerInterceptor(a.log),
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_logrus.UnaryServerInterceptor(a.log),
			grpc_util.PanicLoggerUnaryServerInterceptor(a.log),
		)),
	)
	version.RegisterVersionServiceServer(grpcS, &version.Server{})
	clusterService := cluster.NewServer(a.ns, a.kubeclientset, a.appclientset)
	cluster.RegisterClusterServiceServer(grpcS, clusterService)
	application.RegisterApplicationServiceServer(grpcS, application.NewServer(a.ns, a.kubeclientset, a.appclientset, clusterService))
	repository.RegisterRepositoryServiceServer(grpcS, repository.NewServer(a.ns, a.kubeclientset, a.appclientset))

	// HTTP 1.1+JSON Server
	// grpc-ecosystem/grpc-gateway is used to proxy HTTP requests to the corresponding gRPC call
	mux := http.NewServeMux()
	// NOTE: if a marshaller option is not supplied, grpc-gateway will default to the jsonpb from
	// golang/protobuf. Which does not support types such as time.Time. gogo/protobuf does support
	// time.Time, but does not support custom UnmarshalJSON() and MarshalJSON() methods. Therefore
	// we use our own Marshaler
	gwMuxOpts := runtime.WithMarshalerOption(runtime.MIMEWildcard, new(jsonutil.JSONMarshaler))
	gwmux := runtime.NewServeMux(gwMuxOpts)
	mux.Handle("/api/", gwmux)
	dOpts := []grpc.DialOption{grpc.WithInsecure()}
	mustRegisterGWHandler(version.RegisterVersionServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(cluster.RegisterClusterServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(application.RegisterApplicationServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(repository.RegisterRepositoryServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)

	if a.staticAssetsDir != "" {
		mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			acceptHtml := false
			for _, acceptType := range strings.Split(request.Header.Get("Accept"), ",") {
				if acceptType == "text/html" || acceptType == "html" {
					acceptHtml = true
					break
				}
			}
			fileRequest := request.URL.Path != "/index.html" && strings.Contains(request.URL.Path, ".")

			// serve index.html for non file requests to support HTML5 History API
			if acceptHtml && !fileRequest && (request.Method == "GET" || request.Method == "HEAD") {
				http.ServeFile(writer, request, a.staticAssetsDir+"/index.html")
			} else {
				http.ServeFile(writer, request, a.staticAssetsDir+request.URL.Path)
			}
		})
	}

	httpS := &http.Server{
		Addr:    endpoint,
		Handler: mux,
	}

	// Start the muxed listeners for our servers
	log.Infof("argocd %s serving on port %d (namespace: %s)", argocd.GetVersion(), port, a.ns)
	go func() { _ = grpcS.Serve(grpcL) }()
	go func() { _ = httpS.Serve(httpL) }()
	err = m.Serve()
	if err != nil {
		panic(err)
	}
}

type registerFunc func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error

// mustRegisterGWHandler is a convenience function to register a gateway handler
func mustRegisterGWHandler(register registerFunc, ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) {
	err := register(ctx, mux, endpoint, opts)
	if err != nil {
		panic(err)
	}
}
