package session

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/util/settings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/server/rbacpolicy"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	sessionmgr "github.com/argoproj/argo-cd/v3/util/session"
)

// Server provides a Session service
type Server struct {
	mgr                *sessionmgr.SessionManager
	settingsMgr        *settings.SettingsManager
	authenticator      Authenticator
	policyEnf          *rbacpolicy.RBACPolicyEnforcer
	limitLoginAttempts func() (utilio.Closer, error)
}

type Authenticator interface {
	Authenticate(ctx context.Context) (context.Context, error)
}

const (
	success = "success"
	failure = "failure"
)

// NewServer returns a new instance of the Session service
func NewServer(mgr *sessionmgr.SessionManager, settingsMgr *settings.SettingsManager, authenticator Authenticator, policyEnf *rbacpolicy.RBACPolicyEnforcer, rateLimiter func() (utilio.Closer, error)) *Server {
	return &Server{mgr, settingsMgr, authenticator, policyEnf, rateLimiter}
}

// Create generates a JWT token signed by Argo CD intended for web/CLI logins of the admin user
// using username/password
func (s *Server) Create(_ context.Context, q *session.SessionCreateRequest) (*session.SessionResponse, error) {
	if s.limitLoginAttempts != nil {
		closer, err := s.limitLoginAttempts()
		if err != nil {
			s.mgr.IncLoginRequestCounter(failure)
			return nil, err
		}
		defer utilio.Close(closer)
	}

	if q.Token != "" {
		s.mgr.IncLoginRequestCounter(failure)
		return nil, status.Errorf(codes.Unauthenticated, "token-based session creation no longer supported. please upgrade argocd cli to v0.7+")
	}
	if q.Username == "" || q.Password == "" {
		s.mgr.IncLoginRequestCounter(failure)
		return nil, status.Errorf(codes.Unauthenticated, "no credentials supplied")
	}
	err := s.mgr.VerifyUsernamePassword(q.Username, q.Password)
	if err != nil {
		s.mgr.IncLoginRequestCounter(failure)
		return nil, err
	}
	uniqueId, err := uuid.NewRandom()
	if err != nil {
		s.mgr.IncLoginRequestCounter(failure)
		return nil, err
	}
	argoCDSettings, err := s.settingsMgr.GetSettings()
	if err != nil {
		s.mgr.IncLoginRequestCounter(failure)
		return nil, err
	}
	jwtToken, err := s.mgr.Create(
		fmt.Sprintf("%s:%s", q.Username, settings.AccountCapabilityLogin),
		int64(argoCDSettings.UserSessionDuration.Seconds()),
		uniqueId.String())
	if err != nil {
		s.mgr.IncLoginRequestCounter(failure)
		return nil, err
	}
	s.mgr.IncLoginRequestCounter(success)
	return &session.SessionResponse{Token: jwtToken}, nil
}

// Delete an authentication cookie from the client.  This makes sense only for the Web client.
func (s *Server) Delete(_ context.Context, _ *session.SessionDeleteRequest) (*session.SessionResponse, error) {
	return &session.SessionResponse{Token: ""}, nil
}

// AuthFuncOverride overrides the authentication function and let us not require auth to receive auth.
// Without this function here, ArgoCDServer.authenticate would be invoked and credentials checked.
// Since this service is generally invoked when the user has _no_ credentials, that would create a
// chicken-and-egg situation if we didn't place this here to allow traffic to pass through.
func (s *Server) AuthFuncOverride(ctx context.Context, _ string) (context.Context, error) {
	// this authenticates the user, but ignores any error, so that we have claims populated
	ctx, _ = s.authenticator.Authenticate(ctx)
	return ctx, nil
}

func (s *Server) GetUserInfo(ctx context.Context, _ *session.GetUserInfoRequest) (*session.GetUserInfoResponse, error) {
	return &session.GetUserInfoResponse{
		LoggedIn: sessionmgr.LoggedIn(ctx),
		Username: sessionmgr.Username(ctx),
		Iss:      sessionmgr.Iss(ctx),
		Groups:   sessionmgr.Groups(ctx, s.policyEnf.GetScopes()),
	}, nil
}
