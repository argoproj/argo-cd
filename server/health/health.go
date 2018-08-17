package application

import (
	"fmt"

	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/controller"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/rbac"
)

// Server provides a Health service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
	repoClientset reposerver.Clientset
	db            db.ArgoDB
	appComparator controller.AppStateManager
	enf           *rbac.Enforcer
	projectLock   *util.KeyLock
	auditLogger   *argo.AuditLogger
}

// NewServer returns a new instance of the Health service
func NewServer(
	namespace string,
	kubeclientset kubernetes.Interface,
	appclientset appclientset.Interface,
	repoClientset reposerver.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
	projectLock *util.KeyLock,
) HealthServiceServer {

	return &Server{
		ns:            namespace,
		appclientset:  appclientset,
		kubeclientset: kubeclientset,
		db:            db,
		repoClientset: repoClientset,
		appComparator: controller.NewAppStateManager(db, appclientset, repoClientset, namespace),
		enf:           enf,
		projectLock:   projectLock,
		auditLogger:   argo.NewAuditLogger(namespace, kubeclientset, "argocd-server"),
	}
}

func (s *Server) health(ctx context.Context, healthReq *HealthRequest) (*HealthResponse, error) {
	fmt.Println("HEALTH")
}
