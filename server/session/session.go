package session

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/pkg/apiclient/session"
	sessionmgr "github.com/argoproj/argo-cd/util/session"
)

// Server provides a Session service
type Server struct {
	mgr *sessionmgr.SessionManager
}

// NewServer returns a new instance of the Session service
func NewServer(mgr *sessionmgr.SessionManager) *Server {
	return &Server{
		mgr: mgr,
	}
}

// Create generates a JWT token signed by Argo CD intended for web/CLI logins of the admin user
// using username/password
func (s *Server) Create(ctx context.Context, q *session.SessionCreateRequest) (*session.SessionResponse, error) {
	if q.Token != "" {
		return nil, status.Errorf(codes.Unauthenticated, "token-based session creation no longer supported. please upgrade argocd cli to v0.7+")
	}
	if q.Username == "" || q.Password == "" {
		return nil, status.Errorf(codes.Unauthenticated, "no credentials supplied")
	}
	err := s.mgr.VerifyUsernamePassword(q.Username, q.Password)
	if err != nil {
		return nil, err
	}
	jwtToken, err := s.mgr.Create(q.Username, 0)
	if err != nil {
		return nil, err
	}
	return &session.SessionResponse{Token: jwtToken}, nil
}

// Delete an authentication cookie from the client.  This makes sense only for the Web client.
func (s *Server) Delete(ctx context.Context, q *session.SessionDeleteRequest) (*session.SessionResponse, error) {
	return &session.SessionResponse{Token: ""}, nil
}

// AuthFuncOverride overrides the authentication function and let us not require auth to receive auth.
// Without this function here, ArgoCDServer.authenticate would be invoked and credentials checked.
// Since this service is generally invoked when the user has _no_ credentials, that would create a
// chicken-and-egg situation if we didn't place this here to allow traffic to pass through.
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
