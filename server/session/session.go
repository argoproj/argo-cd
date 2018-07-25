package session

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/util/jwt"
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

// Create generates a non-expiring JWT token signed by ArgoCD. This endpoint is used in two circumstances:
// 1. Web/CLI logins for local users (i.e. admin), for when SSO is not configured. In this case,
//    username/password.
// 2. CLI login which completed an OAuth2 login flow but wish to store a permanent token in their config
func (s *Server) Create(ctx context.Context, q *SessionCreateRequest) (*SessionResponse, error) {
	var tokenString string
	var err error
	if q.Password != "" {
		// first case
		err = s.mgr.VerifyUsernamePassword(q.Username, q.Password)
		if err != nil {
			return nil, err
		}
		tokenString, err = s.mgr.Create(q.Username, 0)
		if err != nil {
			return nil, err
		}
	} else if q.Token != "" {
		// second case
		claimsIf, err := s.mgr.VerifyToken(q.Token)
		if err != nil {
			return nil, err
		}
		claims, err := jwt.MapClaims(claimsIf)
		if err != nil {
			return nil, err
		}
		tokenString, err = s.mgr.ReissueClaims(claims, 0)
		if err != nil {
			return nil, fmt.Errorf("Failed to resign claims: %v", err)
		}
	} else {
		return nil, status.Errorf(codes.Unauthenticated, "no credentials supplied")
	}
	return &SessionResponse{Token: tokenString}, nil
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
