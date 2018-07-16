package account

import (
	jwt "github.com/dgrijalva/jwt-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	jwtutil "github.com/argoproj/argo-cd/util/jwt"
	"github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
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
	username := getAuthenticatedUser(ctx)
	cdSettings, err := s.settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}
	if _, ok := cdSettings.LocalUsers[username]; !ok {
		return nil, status.Errorf(codes.InvalidArgument, "password can only be changed for local users")
	}

	err = s.sessionMgr.VerifyUsernamePassword(username, q.CurrentPassword)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "current password does not match")
	}

	hashedPassword, err := password.HashPassword(q.NewPassword)
	if err != nil {
		return nil, err
	}

	cdSettings.LocalUsers[username] = hashedPassword

	err = s.settingsMgr.SaveSettings(cdSettings)
	if err != nil {
		return nil, err
	}

	return &UpdatePasswordResponse{}, nil

}

// getAuthenticatedUser returns the currently authenticated user (via JWT 'sub' field)
func getAuthenticatedUser(ctx context.Context) string {
	claimsIf := ctx.Value("claims")
	if claimsIf == nil {
		return ""
	}
	claims, ok := claimsIf.(jwt.Claims)
	if !ok {
		return ""
	}
	mapClaims, err := jwtutil.MapClaims(claims)
	if err != nil {
		return ""
	}
	return jwtutil.GetField(mapClaims, "sub")
}
