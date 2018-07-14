package users

import (
	"github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
	"golang.org/x/net/context"
)

// Server provides a Session service
type Server struct {
	sessionMgr  *session.SessionManager
	settingsMgr *settings.SettingsManager
}

// NewServer returns a new instance of the Session service
func NewServer(sessionMgr *session.SessionManager, settingsMgr *settings.SettingsManager) *Server {
	return &Server{
		sessionMgr:  sessionMgr,
		settingsMgr: settingsMgr,
	}

}

//UpdatePassword is used to Update a User's Passwords
func (s *Server) UpdatePassword(ctx context.Context, q *UpdatePasswordRequest) (*UpdatePasswordResponse, error) {

	cdSettings, err := s.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}

	err = s.sessionMgr.VerifyUsernamePassword(q.Name, q.Body.GetCurrentPassword())
	if err != nil {
		return nil, err
	}

	hashedPassword, err := password.HashPassword(q.Body.GetNewPassword())
	if err != nil {
		return nil, err
	}

	cdSettings.LocalUsers[q.Name] = hashedPassword

	err = s.settingsMgr.SaveSettings(cdSettings)
	if err != nil {
		return nil, err
	}

	return nil, nil

}
