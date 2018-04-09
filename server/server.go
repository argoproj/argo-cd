package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strings"

	argocd "github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/errors"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/server/version"
	"github.com/argoproj/argo-cd/util/config"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	jsonutil "github.com/argoproj/argo-cd/util/json"
	util_session "github.com/argoproj/argo-cd/util/session"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	log "github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
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
	ArgoCDServerOpts

	settings config.ArgoCDSettings
	log      *log.Entry
}

type ArgoCDServerOpts struct {
	Insecure        bool
	Namespace       string
	StaticAssetsDir string
	KubeClientset   kubernetes.Interface
	AppClientset    appclientset.Interface
	RepoClientset   reposerver.Clientset
}

// NewServer returns a new instance of the ArgoCD API server
func NewServer(opts ArgoCDServerOpts) *ArgoCDServer {
	configManager := config.NewConfigManager(opts.KubeClientset, opts.Namespace)
	settings, err := configManager.GetSettings()
	if err != nil {
		log.Fatal(err)
	}
	return &ArgoCDServer{
		ArgoCDServerOpts: opts,
		log:              log.NewEntry(log.New()),
		settings:         *settings,
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

	grpcS := a.newGRPCServer()
	var httpS *http.Server
	var httpsS *http.Server
	if a.useTLS() {
		httpS = newRedirectServer()
		httpsS = a.newHTTPServer(ctx)
	} else {
		httpS = a.newHTTPServer(ctx)
	}

	// Cmux is used to support servicing gRPC and HTTP1.1+JSON on the same port
	conn, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	errors.CheckError(err)

	tcpm := cmux.New(conn)
	var tlsm cmux.CMux
	var grpcL net.Listener
	var httpL net.Listener
	var httpsL net.Listener
	if !a.useTLS() {
		httpL = tcpm.Match(cmux.HTTP1Fast())
		grpcL = tcpm.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
	} else {
		// We first match on HTTP 1.1 methods.
		httpL = tcpm.Match(cmux.HTTP1Fast())

		// If not matched, we assume that its TLS.
		tlsl := tcpm.Match(cmux.Any())
		tlsConfig := tls.Config{
			Certificates: []tls.Certificate{*a.settings.Certificate},
		}
		tlsl = tls.NewListener(tlsl, &tlsConfig)

		// Now, we build another mux recursively to match HTTPS and GoRPC.
		tlsm = cmux.New(tlsl)
		httpsL = tlsm.Match(cmux.HTTP1Fast())
		grpcL = tlsm.Match(cmux.Any())
	}

	// Start the muxed listeners for our servers
	log.Infof("argocd %s serving on port %d (tls: %v, namespace: %s)", argocd.GetVersion(), port, a.useTLS(), a.Namespace)
	go func() { errors.CheckError(grpcS.Serve(grpcL)) }()
	go func() { errors.CheckError(httpS.Serve(httpL)) }()
	if a.useTLS() {
		go func() { errors.CheckError(httpsS.Serve(httpsL)) }()
		go func() { errors.CheckError(tlsm.Serve()) }()
	}
	err = tcpm.Serve()
	errors.CheckError(err)
}

func (a *ArgoCDServer) useTLS() bool {
	if a.Insecure || a.settings.Certificate == nil {
		return false
	}
	return true
}

func (a *ArgoCDServer) newGRPCServer() *grpc.Server {
	var sOpts []grpc.ServerOption
	// NOTE: notice we do not configure the gRPC server here with TLS (e.g. grpc.Creds(creds))
	// This is because TLS handshaking occurs in cmux handling
	sOpts = append(sOpts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
		grpc_logrus.StreamServerInterceptor(a.log),
		grpc_auth.StreamServerInterceptor(a.authenticate),
		grpc_util.PanicLoggerStreamServerInterceptor(a.log),
	)))
	sOpts = append(sOpts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		grpc_logrus.UnaryServerInterceptor(a.log),
		grpc_auth.UnaryServerInterceptor(a.authenticate),
		grpc_util.PanicLoggerUnaryServerInterceptor(a.log),
	)))

	grpcS := grpc.NewServer(sOpts...)
	clusterService := cluster.NewServer(a.Namespace, a.KubeClientset, a.AppClientset)
	repoService := repository.NewServer(a.Namespace, a.KubeClientset, a.AppClientset)
	sessionService := session.NewServer(a.Namespace, a.KubeClientset, a.AppClientset, a.settings)
	applicationService := application.NewServer(a.Namespace, a.KubeClientset, a.AppClientset, a.RepoClientset, repoService, clusterService)
	version.RegisterVersionServiceServer(grpcS, &version.Server{})
	cluster.RegisterClusterServiceServer(grpcS, clusterService)
	application.RegisterApplicationServiceServer(grpcS, applicationService)
	repository.RegisterRepositoryServiceServer(grpcS, repoService)
	session.RegisterSessionServiceServer(grpcS, sessionService)
	return grpcS
}

// newHTTPServer returns the HTTP server to serve HTTP/HTTPS requests. This is implemented
// using grpc-gateway as a proxy to the gRPC server.
func (a *ArgoCDServer) newHTTPServer(ctx context.Context) *http.Server {
	mux := http.NewServeMux()
	httpS := http.Server{
		Addr:    endpoint,
		Handler: mux,
	}
	var dOpts []grpc.DialOption
	if a.useTLS() {
		// The following sets up the dial Options for grpc-gateway to talk to gRPC server over TLS.
		// grpc-gateway is just translating HTTP/HTTPS requests as gRPC requests over localhost,
		// so we need to supply the same certificates to establish the connections that a normal,
		// external gRPC client would need.
		certPool := x509.NewCertPool()
		pemCertBytes, _ := tlsutil.EncodeX509KeyPair(*a.settings.Certificate)
		ok := certPool.AppendCertsFromPEM(pemCertBytes)
		if !ok {
			panic("bad certs")
		}
		dCreds := credentials.NewTLS(&tls.Config{
			RootCAs:            certPool,
			InsecureSkipVerify: true,
		})
		dOpts = append(dOpts, grpc.WithTransportCredentials(dCreds))
	} else {
		dOpts = append(dOpts, grpc.WithInsecure())
	}

	// HTTP 1.1+JSON Server
	// grpc-ecosystem/grpc-gateway is used to proxy HTTP requests to the corresponding gRPC call
	// NOTE: if a marshaller option is not supplied, grpc-gateway will default to the jsonpb from
	// golang/protobuf. Which does not support types such as time.Time. gogo/protobuf does support
	// time.Time, but does not support custom UnmarshalJSON() and MarshalJSON() methods. Therefore
	// we use our own Marshaler
	gwMuxOpts := runtime.WithMarshalerOption(runtime.MIMEWildcard, new(jsonutil.JSONMarshaler))
	gwmux := runtime.NewServeMux(gwMuxOpts)
	mux.Handle("/api/", gwmux)
	mustRegisterGWHandler(version.RegisterVersionServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(cluster.RegisterClusterServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(application.RegisterApplicationServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(repository.RegisterRepositoryServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(session.RegisterSessionServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)

	if a.StaticAssetsDir != "" {
		mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			acceptHTML := false
			for _, acceptType := range strings.Split(request.Header.Get("Accept"), ",") {
				if acceptType == "text/html" || acceptType == "html" {
					acceptHTML = true
					break
				}
			}
			fileRequest := request.URL.Path != "/index.html" && strings.Contains(request.URL.Path, ".")

			// serve index.html for non file requests to support HTML5 History API
			if acceptHTML && !fileRequest && (request.Method == "GET" || request.Method == "HEAD") {
				http.ServeFile(writer, request, a.StaticAssetsDir+"/index.html")
			} else {
				http.ServeFile(writer, request, a.StaticAssetsDir+request.URL.Path)
			}
		})
	}
	return &httpS
}

// newRedirectServer returns an HTTP server which does a 307 redirect to the HTTPS server
func newRedirectServer() *http.Server {
	return &http.Server{
		Addr: endpoint,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			target := "https://" + req.Host + req.URL.Path
			if len(req.URL.RawQuery) > 0 {
				target += "?" + req.URL.RawQuery
			}
			log.Printf("redirect to: %s", target)
			http.Redirect(w, req, target, http.StatusTemporaryRedirect)
		}),
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

// Authenticate checks for the presence of a token when accessing server-side resources.
func (a *ArgoCDServer) authenticate(ctx context.Context) (context.Context, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		mgr := util_session.MakeSessionManager(a.settings.ServerSignature)
		tokens := md["tokens"]
		for _, token := range tokens {
			_, err := mgr.Parse(token)
			if err == nil {
				return ctx, nil
			}
		}
		return ctx, fmt.Errorf("user is not allowed access")
	}

	return ctx, fmt.Errorf("empty metadata")
}
