package account

import (
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/kubectl/util/slice"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

// Server provides a Session service
type Server struct {
	sessionMgr  *session.SessionManager
	settingsMgr *settings.SettingsManager
	enf         *rbac.Enforcer
}

// NewServer returns a new instance of the Session service
func NewServer(sessionMgr *session.SessionManager, settingsMgr *settings.SettingsManager, enf *rbac.Enforcer) *Server {
	return &Server{sessionMgr, settingsMgr, enf}

}

// UpdatePassword updates the password of the local admin superuser.
func (s *Server) UpdatePassword(ctx context.Context, q *account.UpdatePasswordRequest) (*account.UpdatePasswordResponse, error) {
	sub := session.Sub(ctx)
	if sub != common.ArgoCDAdminUsername {
		return nil, status.Errorf(codes.InvalidArgument, "password can only be changed for local users, not user %q", sub)
	}

	cdSettings, err := s.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}

	err = s.sessionMgr.VerifyUsernamePassword(sub, q.CurrentPassword)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "current password does not match")
	}

	hashedPassword, err := password.HashPassword(q.NewPassword)
	if err != nil {
		return nil, err
	}

	cdSettings.AdminPasswordHash = hashedPassword
	cdSettings.AdminPasswordMtime = time.Now().UTC()

	err = s.settingsMgr.SaveSettings(cdSettings)
	if err != nil {
		return nil, err
	}
	log.Infof("user '%s' updated password", sub)
	return &account.UpdatePasswordResponse{}, nil

}

func (s *Server) CanI(ctx context.Context, r *account.CanIRequest) (*account.CanIResponse, error) {
	if !slice.ContainsString(rbacpolicy.Actions, r.Action, nil) {
		return nil, status.Errorf(codes.InvalidArgument, "%v does not contain %s", rbacpolicy.Actions, r.Action)
	}
	if !slice.ContainsString(rbacpolicy.Resources, r.Resource, nil) {
		return nil, status.Errorf(codes.InvalidArgument, "%v does not contain %s", rbacpolicy.Resources, r.Resource)
	}
	ok := s.enf.Enforce(ctx.Value("claims"), r.Resource, r.Action, r.Subresource)
	if ok {
		return &account.CanIResponse{Value: "yes"}, nil
	} else {
		return &account.CanIResponse{Value: "no"}, nil
	}
}
