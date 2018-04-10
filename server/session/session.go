package session

import (
	"context"

	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/session"
	"k8s.io/client-go/kubernetes"
)

// Server provides a Session service
type Server struct {
	ns             string
	kubeclientset  kubernetes.Interface
	appclientset   appclientset.Interface
	serversettings config.ArgoCDSettings
}

// NewServer returns a new instance of the Session service
func NewServer(namespace string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface, serversettings config.ArgoCDSettings) *Server {
	return &Server{
		ns:             namespace,
		appclientset:   appclientset,
		kubeclientset:  kubeclientset,
		serversettings: serversettings,
	}
}

// Create a a JWT for authentication.
func (s *Server) Create(ctx context.Context, q *SessionRequest) (*SessionResponse, error) {
	sessionManager := session.MakeSessionManager(s.serversettings.ServerSignature)
	token, err := sessionManager.LoginLocalUser(q.Username, q.Password, s.serversettings.LocalUsers)

	return &SessionResponse{token}, err
}

// AuthFuncOverride overrides the authentication function and let us not require auth to receive auth.
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
