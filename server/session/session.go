package session

import (
	"context"

	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/session"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// invalidLoginMessage, for security purposes, doesn't say whether the username or password was invalid.  This does not mitigate the potential for timing attacks to determine which is which.
const (
	invalidLoginError  = "Invalid username or password"
	blankPasswordError = "Blank passwords are not allowed"
)

// Create an authentication cookie for the client.
func (s *Server) Create(ctx context.Context, q *SessionCreateRequest) (*SessionResponse, error) {
	if q.Password == "" {
		err := status.Errorf(codes.Unauthenticated, blankPasswordError)
		return nil, err
	}

	passwordHash, ok := s.serversettings.LocalUsers[q.Username]
	if !ok {
		// Username was not found in local user store.
		// Ensure we still send password to hashing algorithm for comparison.
		// This mitigates potential for timing attacks that benefit from short-circuiting,
		// provided the hashing library/algorithm in use doesn't itself short-circuit.
		passwordHash = ""
	}

	valid, _ := password.VerifyPassword(q.Password, passwordHash)
	if !valid {
		err := status.Errorf(codes.Unauthenticated, invalidLoginError)
		return nil, err
	}

	sessionManager := session.MakeSessionManager(s.serversettings.ServerSignature)
	token, err := sessionManager.Create(q.Username)
	if err != nil {
		token = ""
	}

	return &SessionResponse{token}, err
}

// Delete an authentication cookie from the client.  This makes sense only for the Web client.
func (s *Server) Delete(ctx context.Context, q *SessionDeleteRequest) (*SessionResponse, error) {
	return &SessionResponse{""}, nil
}

// AuthFuncOverride overrides the authentication function and let us not require auth to receive auth.
// Without this function here, ArgoCDServer.authenticate would be invoked and credentials checked.
// Since this service is generally invoked when the user has _no_ credentials, that would create a
// chicken-and-egg situation if we didn't place this here to allow traffic to pass through.
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
