package session

import (
	"context"
	"fmt"

	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util"
	"k8s.io/client-go/kubernetes"
)

// Server provides a Session service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
}

// NewServer returns a new instance of the Session service
func NewServer(namespace string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface) *Server {
	return &Server{
		ns:            namespace,
		appclientset:  appclientset,
		kubeclientset: kubeclientset,
	}
}

// invalidLoginMessage, for security purposes, doesn't say whether the username or password was invalid.  This does not mitigate the potential for timing attacks to determine which is which.
const invalidLoginMessage = "Invalid username or password"

// Create a a JWT for authentication.
func (s *Server) Create(ctx context.Context, q *SessionRequest) (*SessionResponse, error) {
	configMapName := "hello"
	config := util.NewConfigManager(s.kubeclientset, s.ns, configMapName)
	settings, err := config.GetSettings()

	if err != nil {
		return nil, err
	}

	passwordHash, ok := settings.LocalUsers[q.Username]
	if !ok {
		// Username was not found in local user store.
		// Ensure we still send password to hashing algorithm for comparison.
		// This mitigates potential for timing attacks that depend on short-circuiting,
		// provided the hashing library/algorithm in use doesn't itself short-circuit.
		passwordHash = ""
	}

	valid, _ := util.VerifyPassword(q.Password, passwordHash)
	if !valid {
		err = fmt.Errorf(invalidLoginMessage)
		return nil, err
	}

	mgr := util.MakeSessionManager("this should not be here")
	token, err := mgr.Create(q.Username)
	if err != nil {
		token = ""
	}
	return &SessionResponse{token}, err
}
