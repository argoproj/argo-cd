package session

import (
	"context"
	"fmt"

	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/password"
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

// invalidLoginMessage, for security purposes, doesn't say whether the username or password was invalid.  This does not mitigate the potential for timing attacks to determine which is which.
const (
	invalidLoginError  = "Invalid username or password"
	blankPasswordError = "Blank passwords are not allowed"
)

// Create a a JWT for authentication.
func (s *Server) Create(ctx context.Context, q *SessionRequest) (*SessionResponse, error) {
	if q.Password == "" {
		err := fmt.Errorf(blankPasswordError)
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
		err := fmt.Errorf(invalidLoginError)
		return nil, err
	}

	sessionManager := session.MakeSessionManager(s.serversettings.ServerSignature)
	token, err := sessionManager.Create(q.Username)
	if err != nil {
		token = ""
	}
	return &SessionResponse{token}, err
}
