package account

import (
	"context"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/account"
	sessionpkg "github.com/argoproj/argo-cd/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/rbac"
	sessionutil "github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	testNamespace = "default"
)

// return an AccountServer which returns fake data
func newTestAccountServer(ctx context.Context, opts ...func(cm *v1.ConfigMap, secret *v1.Secret)) (*Server, *session.Server) {
	return newTestAccountServerExt(ctx, func(claims jwt.Claims, rvals ...interface{}) bool {
		return true
	}, opts...)
}

func newTestAccountServerExt(ctx context.Context, enforceFn rbac.ClaimsEnforcerFunc, opts ...func(cm *v1.ConfigMap, secret *v1.Secret)) (*Server, *session.Server) {
	bcrypt, err := password.HashPassword("oldpassword")
	errors.CheckError(err)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{},
	}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte(bcrypt),
			"server.secretkey": []byte("test"),
		},
	}
	for i := range opts {
		opts[i](cm, secret)
	}
	kubeclientset := fake.NewSimpleClientset(cm, secret)
	settingsMgr := settings.NewSettingsManager(ctx, kubeclientset, testNamespace)
	sessionMgr := sessionutil.NewSessionManager(settingsMgr, "", sessionutil.NewInMemoryUserStateStorage())
	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	enforcer.SetClaimsEnforcerFunc(enforceFn)

	return NewServer(sessionMgr, settingsMgr, enforcer), session.NewServer(sessionMgr, nil, nil, nil)
}

func getAdminAccount(mgr *settings.SettingsManager) (*settings.Account, error) {
	accounts, err := mgr.GetAccounts()
	if err != nil {
		return nil, err
	}
	adminAccount := accounts[common.ArgoCDAdminUsername]
	return &adminAccount, nil
}

func adminContext(ctx context.Context) context.Context {
	// nolint:staticcheck
	return context.WithValue(ctx, "claims", &jwt.StandardClaims{Subject: "admin", Issuer: sessionutil.SessionManagerClaimsIssuer})
}

func ssoAdminContext(ctx context.Context, iat time.Time) context.Context {
	// nolint:staticcheck
	return context.WithValue(ctx, "claims", &jwt.StandardClaims{
		Subject:  "admin",
		Issuer:   "https://myargocdhost.com/api/dex",
		IssuedAt: iat.Unix(),
	})
}

func projTokenContext(ctx context.Context) context.Context {
	// nolint:staticcheck
	return context.WithValue(ctx, "claims", &jwt.StandardClaims{
		Subject: "proj:demo:deployer",
		Issuer:  sessionutil.SessionManagerClaimsIssuer,
	})
}

func TestUpdatePassword(t *testing.T) {
	accountServer, sessionServer := newTestAccountServer(context.Background())
	ctx := adminContext(context.Background())
	var err error

	// ensure password is not allowed to be updated if given bad password
	_, err = accountServer.UpdatePassword(ctx, &account.UpdatePasswordRequest{CurrentPassword: "badpassword", NewPassword: "newpassword"})
	assert.Error(t, err)
	assert.NoError(t, accountServer.sessionMgr.VerifyUsernamePassword("admin", "oldpassword"))
	assert.Error(t, accountServer.sessionMgr.VerifyUsernamePassword("admin", "newpassword"))
	// verify old password works
	_, err = sessionServer.Create(ctx, &sessionpkg.SessionCreateRequest{Username: "admin", Password: "oldpassword"})
	assert.NoError(t, err)
	// verify new password doesn't
	_, err = sessionServer.Create(ctx, &sessionpkg.SessionCreateRequest{Username: "admin", Password: "newpassword"})
	assert.Error(t, err)

	// ensure password can be updated with valid password and immediately be used
	adminAccount, err := getAdminAccount(accountServer.settingsMgr)
	assert.NoError(t, err)
	prevHash := adminAccount.PasswordHash
	_, err = accountServer.UpdatePassword(ctx, &account.UpdatePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword"})
	assert.NoError(t, err)
	adminAccount, err = getAdminAccount(accountServer.settingsMgr)
	assert.NoError(t, err)
	assert.NotEqual(t, prevHash, adminAccount.PasswordHash)
	assert.NoError(t, accountServer.sessionMgr.VerifyUsernamePassword("admin", "newpassword"))
	assert.Error(t, accountServer.sessionMgr.VerifyUsernamePassword("admin", "oldpassword"))
	// verify old password is invalid
	_, err = sessionServer.Create(ctx, &sessionpkg.SessionCreateRequest{Username: "admin", Password: "oldpassword"})
	assert.Error(t, err)
	// verify new password works
	_, err = sessionServer.Create(ctx, &sessionpkg.SessionCreateRequest{Username: "admin", Password: "newpassword"})
	assert.NoError(t, err)
}

func TestUpdatePassword_AdminUpdatesAnotherUser(t *testing.T) {
	accountServer, sessionServer := newTestAccountServer(context.Background(), func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.anotherUser"] = "login"
	})
	ctx := adminContext(context.Background())

	_, err := accountServer.UpdatePassword(ctx, &account.UpdatePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword", Name: "anotherUser"})
	assert.NoError(t, err)

	_, err = sessionServer.Create(ctx, &sessionpkg.SessionCreateRequest{Username: "anotherUser", Password: "newpassword"})
	assert.NoError(t, err)
}

func TestUpdatePassword_DoesNotHavePermissions(t *testing.T) {
	enforcer := func(claims jwt.Claims, rvals ...interface{}) bool {
		return false
	}

	t.Run("LocalAccountUpdatesAnotherAccount", func(t *testing.T) {
		accountServer, _ := newTestAccountServerExt(context.Background(), enforcer, func(cm *v1.ConfigMap, secret *v1.Secret) {
			cm.Data["accounts.anotherUser"] = "login"
		})
		ctx := adminContext(context.Background())
		_, err := accountServer.UpdatePassword(ctx, &account.UpdatePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword", Name: "anotherUser"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("SSOAccountWithTheSameName", func(t *testing.T) {
		accountServer, _ := newTestAccountServerExt(context.Background(), enforcer)
		ctx := ssoAdminContext(context.Background(), time.Now())
		_, err := accountServer.UpdatePassword(ctx, &account.UpdatePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword", Name: "admin"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})
}

func TestUpdatePassword_ProjectToken(t *testing.T) {
	accountServer, _ := newTestAccountServer(context.Background(), func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.anotherUser"] = "login"
	})
	ctx := projTokenContext(context.Background())
	_, err := accountServer.UpdatePassword(ctx, &account.UpdatePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password can only be changed for local users")
}

func TestUpdatePassword_OldSSOToken(t *testing.T) {
	accountServer, _ := newTestAccountServer(context.Background(), func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.anotherUser"] = "login"
	})
	ctx := ssoAdminContext(context.Background(), time.Now().Add(-2*common.ChangePasswordSSOTokenMaxAge))

	_, err := accountServer.UpdatePassword(ctx, &account.UpdatePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword", Name: "anotherUser"})
	assert.Error(t, err)
}

func TestUpdatePassword_SSOUserUpdatesAnotherUser(t *testing.T) {
	accountServer, sessionServer := newTestAccountServer(context.Background(), func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.anotherUser"] = "login"
	})
	ctx := ssoAdminContext(context.Background(), time.Now())

	_, err := accountServer.UpdatePassword(ctx, &account.UpdatePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword", Name: "anotherUser"})
	assert.NoError(t, err)

	_, err = sessionServer.Create(ctx, &sessionpkg.SessionCreateRequest{Username: "anotherUser", Password: "newpassword"})
	assert.NoError(t, err)
}

func TestListAccounts_NoAccountsConfigured(t *testing.T) {
	ctx := adminContext(context.Background())

	accountServer, _ := newTestAccountServer(ctx)
	resp, err := accountServer.ListAccounts(ctx, &account.ListAccountRequest{})
	assert.NoError(t, err)
	assert.Len(t, resp.Items, 1)
}

func TestListAccounts_AccountsAreConfigured(t *testing.T) {
	ctx := adminContext(context.Background())
	accountServer, _ := newTestAccountServer(ctx, func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.account1"] = "apiKey"
		cm.Data["accounts.account2"] = "login, apiKey"
		cm.Data["accounts.account2.enabled"] = "false"
	})

	resp, err := accountServer.ListAccounts(ctx, &account.ListAccountRequest{})
	assert.NoError(t, err)
	assert.Len(t, resp.Items, 3)
	assert.ElementsMatch(t, []*account.Account{
		{Name: "admin", Capabilities: []string{"login"}, Enabled: true},
		{Name: "account1", Capabilities: []string{"apiKey"}, Enabled: true},
		{Name: "account2", Capabilities: []string{"login", "apiKey"}, Enabled: false},
	}, resp.Items)
}

func TestGetAccount(t *testing.T) {
	ctx := adminContext(context.Background())
	accountServer, _ := newTestAccountServer(ctx, func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.account1"] = "apiKey"
	})

	t.Run("ExistingAccount", func(t *testing.T) {
		acc, err := accountServer.GetAccount(ctx, &account.GetAccountRequest{Name: "account1"})
		assert.NoError(t, err)

		assert.Equal(t, acc.Name, "account1")
	})

	t.Run("NonExistingAccount", func(t *testing.T) {
		_, err := accountServer.GetAccount(ctx, &account.GetAccountRequest{Name: "bad-name"})
		assert.Error(t, err)
		assert.Equal(t, status.Code(err), codes.NotFound)
	})
}

func TestCreateToken_SuccessfullyCreated(t *testing.T) {
	ctx := adminContext(context.Background())
	accountServer, _ := newTestAccountServer(ctx, func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.account1"] = "apiKey"
	})

	_, err := accountServer.CreateToken(ctx, &account.CreateTokenRequest{Name: "account1"})
	assert.NoError(t, err)

	acc, err := accountServer.GetAccount(ctx, &account.GetAccountRequest{Name: "account1"})
	assert.NoError(t, err)

	assert.Len(t, acc.Tokens, 1)
}

func TestCreateToken_DoesNotHaveCapability(t *testing.T) {
	ctx := adminContext(context.Background())
	accountServer, _ := newTestAccountServer(ctx, func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.account1"] = "login"
	})

	_, err := accountServer.CreateToken(ctx, &account.CreateTokenRequest{Name: "account1"})
	assert.Error(t, err)
}

func TestCreateToken_UserSpecifiedID(t *testing.T) {
	ctx := adminContext(context.Background())
	accountServer, _ := newTestAccountServer(ctx, func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.account1"] = "apiKey"
	})

	_, err := accountServer.CreateToken(ctx, &account.CreateTokenRequest{Name: "account1", Id: "test"})
	assert.NoError(t, err)

	_, err = accountServer.CreateToken(ctx, &account.CreateTokenRequest{Name: "account1", Id: "test"})
	if !assert.Error(t, err) {
		return
	}
	assert.Contains(t, "account already has token with id 'test'", err.Error())
}

func TestDeleteToken_SuccessfullyRemoved(t *testing.T) {
	ctx := adminContext(context.Background())
	accountServer, _ := newTestAccountServer(ctx, func(cm *v1.ConfigMap, secret *v1.Secret) {
		cm.Data["accounts.account1"] = "apiKey"
		secret.Data["accounts.account1.tokens"] = []byte(`[{"id":"123","iat":1583789194,"exp":1583789194}]`)
	})

	_, err := accountServer.DeleteToken(ctx, &account.DeleteTokenRequest{Name: "account1", Id: "123"})
	assert.NoError(t, err)

	acc, err := accountServer.GetAccount(ctx, &account.GetAccountRequest{Name: "account1"})
	assert.NoError(t, err)

	assert.Len(t, acc.Tokens, 0)
}
