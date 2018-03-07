package reposerver

import (
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/version"
	"github.com/argoproj/argo-cd/util/git"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
)

// ArgoCDRepoServer is the repo server implementation
type ArgoCDRepoServer struct {
	ns            string
	kubeclientset kubernetes.Interface
	log           *log.Entry
}

// NewServer returns a new instance of the ArgoCD Repo server
func NewServer(kubeclientset kubernetes.Interface, namespace string) *ArgoCDRepoServer {
	return &ArgoCDRepoServer{
		ns:            namespace,
		kubeclientset: kubeclientset,
		log:           log.NewEntry(log.New()),
	}
}

// CreateGRPC creates new configured grpc server
func (a *ArgoCDRepoServer) CreateGRPC(gitClient git.Client) *grpc.Server {
	server := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_logrus.StreamServerInterceptor(a.log),
			grpc_util.PanicLoggerStreamServerInterceptor(a.log),
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_logrus.UnaryServerInterceptor(a.log),
			grpc_util.PanicLoggerUnaryServerInterceptor(a.log),
		)),
	)
	version.RegisterVersionServiceServer(server, &version.Server{})
	manifestService := repository.NewService(a.ns, a.kubeclientset, gitClient)
	repository.RegisterRepositoryServiceServer(server, manifestService)

	return server
}
