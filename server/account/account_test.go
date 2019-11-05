package account

import (
	"context"
	"testing"

	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/pkg/apiclient/account"
	sessionpkg "github.com/argoproj/argo-cd/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/util/password"
	sessionutil "github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	testNamespace = "default"
)

// return an AccountServer which returns fake data
func newTestAccountServer(ctx context.Context) (*fake.Clientset, *Server, *session.Server) {
	bcrypt, err := password.HashPassword("oldpassword")
	errors.CheckError(err)
	kubeclientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte(bcrypt),
			"server.secretkey": []byte("test"),
		},
	})
	settingsMgr := settings.NewSettingsManager(ctx, kubeclientset, testNamespace)
	sessionMgr := sessionutil.NewSessionManager(settingsMgr, "")
	return kubeclientset, NewServer(sessionMgr, settingsMgr, nil), session.NewServer(sessionMgr, nil)
}

func TestUpdatePassword(t *testing.T) {
	ctx := context.Background()
	_, accountServer, sessionServer := newTestAccountServer(ctx)
	ctx = context.WithValue(ctx, "claims", &jwt.StandardClaims{Subject: "admin"})
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
	settings, err := accountServer.settingsMgr.GetSettings()
	assert.NoError(t, err)
	prevHash := settings.AdminPasswordHash
	_, err = accountServer.UpdatePassword(ctx, &account.UpdatePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword"})
	assert.NoError(t, err)
	settings, err = accountServer.settingsMgr.GetSettings()
	assert.NoError(t, err)
	assert.NotEqual(t, prevHash, settings.AdminPasswordHash)
	assert.NoError(t, accountServer.sessionMgr.VerifyUsernamePassword("admin", "newpassword"))
	assert.Error(t, accountServer.sessionMgr.VerifyUsernamePassword("admin", "oldpassword"))
	// verify old password is invalid
	_, err = sessionServer.Create(ctx, &sessionpkg.SessionCreateRequest{Username: "admin", Password: "oldpassword"})
	assert.Error(t, err)
	// verify new password works
	_, err = sessionServer.Create(ctx, &sessionpkg.SessionCreateRequest{Username: "admin", Password: "newpassword"})
	assert.NoError(t, err)
}
