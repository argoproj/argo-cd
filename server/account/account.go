package account

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubectl/pkg/util/slice"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/account"
	"github.com/argoproj/argo-cd/v3/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v3/util/password"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/session"
	"github.com/argoproj/argo-cd/v3/util/settings"
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

// UpdatePassword updates the password of the currently authenticated account or the account specified in the request.
func (s *Server) UpdatePassword(ctx context.Context, q *account.UpdatePasswordRequest) (*account.UpdatePasswordResponse, error) {
	username := session.GetUserIdentifier(ctx)

	updatedUsername := username
	if q.Name != "" {
		updatedUsername = q.Name
	}

	// check for permission is user is trying to change someone else's password
	// assuming user is trying to update someone else if username is different or issuer is not Argo CD
	issuer := session.Iss(ctx)
	if updatedUsername != username || issuer != session.SessionManagerClaimsIssuer {
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceAccounts, rbac.ActionUpdate, q.Name); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	if issuer == session.SessionManagerClaimsIssuer {
		// local user is changing own password or another user password

		// user is changing own password.
		// ensure token belongs to a user, not project
		if q.Name == "" && rbacpolicy.IsProjectSubject(username) {
			return nil, status.Errorf(codes.InvalidArgument, "password can only be changed for local users, not user %q", username)
		}

		err := s.sessionMgr.VerifyUsernamePassword(username, q.CurrentPassword)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "current password does not match")
		}
	} else {
		// SSO user is changing or local user password

		iat, err := session.Iat(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get issue time: %w", err)
		}
		if time.Since(iat) > common.ChangePasswordSSOTokenMaxAge {
			return nil, errors.New("SSO token is too old. Please use 'argocd relogin' to get a new token")
		}
	}

	// Need to validate password complexity with regular expression
	passwordPattern, err := s.settingsMgr.GetPasswordPattern()
	if err != nil {
		return nil, fmt.Errorf("failed to get password pattern: %w", err)
	}

	validPasswordRegexp, err := regexp.Compile(passwordPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile password regex: %w", err)
	}

	if !validPasswordRegexp.Match([]byte(q.NewPassword)) {
		err := fmt.Errorf("new password does not match the following expression: %s", passwordPattern)
		return nil, err
	}

	hashedPassword, err := password.HashPassword(q.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	err = s.settingsMgr.UpdateAccount(updatedUsername, func(acc *settings.Account) error {
		acc.PasswordHash = hashedPassword
		now := time.Now().UTC()
		acc.PasswordMtime = &now
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update account password: %w", err)
	}

	if updatedUsername == username {
		log.Infof("user '%s' updated password", username)
	} else {
		log.Infof("user '%s' updated password of user '%s'", username, updatedUsername)
	}
	return &account.UpdatePasswordResponse{}, nil
}

// CanI checks if the current account has permission to perform an action
func (s *Server) CanI(ctx context.Context, r *account.CanIRequest) (*account.CanIResponse, error) {
	if !slice.ContainsString(rbac.Actions, r.Action, nil) {
		return nil, status.Errorf(codes.InvalidArgument, "%v does not contain %s", rbac.Actions, r.Action)
	}
	if !slice.ContainsString(rbac.Resources, r.Resource, nil) {
		return nil, status.Errorf(codes.InvalidArgument, "%v does not contain %s", rbac.Resources, r.Resource)
	}

	ok := s.enf.Enforce(ctx.Value("claims"), r.Resource, r.Action, r.Subresource)
	if ok {
		return &account.CanIResponse{Value: "yes"}, nil
	}
	return &account.CanIResponse{Value: "no"}, nil
}

func toAPIAccount(name string, a settings.Account) *account.Account {
	var capabilities []string
	for _, c := range a.Capabilities {
		capabilities = append(capabilities, string(c))
	}
	var tokens []*account.Token
	for _, t := range a.Tokens {
		tokens = append(tokens, &account.Token{Id: t.ID, ExpiresAt: t.ExpiresAt, IssuedAt: t.IssuedAt})
	}
	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].IssuedAt > tokens[j].IssuedAt
	})
	return &account.Account{
		Name:         name,
		Enabled:      a.Enabled,
		Capabilities: capabilities,
		Tokens:       tokens,
	}
}

func (s *Server) ensureHasAccountPermission(ctx context.Context, action string, account string) error {
	id := session.GetUserIdentifier(ctx)

	// account has always has access to itself
	if id == account && session.Iss(ctx) == session.SessionManagerClaimsIssuer {
		return nil
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceAccounts, action, account); err != nil {
		return fmt.Errorf("permission denied for account %s with action %s: %w", account, action, err)
	}
	return nil
}

// ListAccounts returns the list of accounts
func (s *Server) ListAccounts(ctx context.Context, _ *account.ListAccountRequest) (*account.AccountsList, error) {
	resp := account.AccountsList{}
	accounts, err := s.settingsMgr.GetAccounts()
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}
	for name, a := range accounts {
		if err := s.ensureHasAccountPermission(ctx, rbac.ActionGet, name); err == nil {
			resp.Items = append(resp.Items, toAPIAccount(name, a))
		}
	}
	sort.Slice(resp.Items, func(i, j int) bool {
		return resp.Items[i].Name < resp.Items[j].Name
	})
	return &resp, nil
}

// GetAccount returns an account
func (s *Server) GetAccount(ctx context.Context, r *account.GetAccountRequest) (*account.Account, error) {
	if err := s.ensureHasAccountPermission(ctx, rbac.ActionGet, r.Name); err != nil {
		return nil, fmt.Errorf("permission denied to get account %s: %w", r.Name, err)
	}
	a, err := s.settingsMgr.GetAccount(r.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get account %s: %w", r.Name, err)
	}
	return toAPIAccount(r.Name, *a), nil
}

// CreateToken creates a token
func (s *Server) CreateToken(ctx context.Context, r *account.CreateTokenRequest) (*account.CreateTokenResponse, error) {
	if err := s.ensureHasAccountPermission(ctx, rbac.ActionUpdate, r.Name); err != nil {
		return nil, fmt.Errorf("permission denied to create token for account %s: %w", r.Name, err)
	}

	id := r.Id
	if id == "" {
		uniqueId, err := uuid.NewRandom()
		if err != nil {
			return nil, fmt.Errorf("failed to generate unique ID: %w", err)
		}
		id = uniqueId.String()
	}

	var tokenString string
	err := s.settingsMgr.UpdateAccount(r.Name, func(account *settings.Account) error {
		if account.TokenIndex(id) > -1 {
			return fmt.Errorf("account already has token with id '%s'", id)
		}
		if !account.HasCapability(settings.AccountCapabilityApiKey) {
			return fmt.Errorf("account '%s' does not have %s capability", r.Name, settings.AccountCapabilityApiKey)
		}

		now := time.Now()
		var err error
		tokenString, err = s.sessionMgr.Create(fmt.Sprintf("%s:%s", r.Name, settings.AccountCapabilityApiKey), r.ExpiresIn, id)
		if err != nil {
			return err
		}

		var expiresAt int64
		if r.ExpiresIn > 0 {
			expiresAt = now.Add(time.Duration(r.ExpiresIn) * time.Second).Unix()
		}
		account.Tokens = append(account.Tokens, settings.Token{
			ID:        id,
			IssuedAt:  now.Unix(),
			ExpiresAt: expiresAt,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update account with new token: %w", err)
	}
	return &account.CreateTokenResponse{Token: tokenString}, nil
}

// DeleteToken deletes a token
func (s *Server) DeleteToken(ctx context.Context, r *account.DeleteTokenRequest) (*account.EmptyResponse, error) {
	if err := s.ensureHasAccountPermission(ctx, rbac.ActionUpdate, r.Name); err != nil {
		return nil, fmt.Errorf("permission denied to delete account %s: %w", r.Name, err)
	}

	err := s.settingsMgr.UpdateAccount(r.Name, func(account *settings.Account) error {
		if index := account.TokenIndex(r.Id); index > -1 {
			account.Tokens = append(account.Tokens[:index], account.Tokens[index+1:]...)
			return nil
		}
		return status.Errorf(codes.NotFound, "token with id '%s' does not exist", r.Id)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete account %s: %w", r.Name, err)
	}
	return &account.EmptyResponse{}, nil
}
