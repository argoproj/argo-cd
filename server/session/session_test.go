package session

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	sessionpb "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/password"
	utilrbac "github.com/argoproj/argo-cd/v3/util/rbac"
	sessionmgr "github.com/argoproj/argo-cd/v3/util/session"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	testNamespace  = "argocd"
	testAdminPass  = "test-password"
	testSecretKey  = "test-secret-key"
	testRBACCMName = "argocd-rbac-cm"
)

// newTestKubeClient creates a fake clientset with argocd-cm and argocd-secret
// configured for the built-in admin account.
func newTestKubeClient(t *testing.T) *fake.Clientset {
	t.Helper()
	hashed, err := password.HashPassword(testAdminPass)
	require.NoError(t, err)
	return fake.NewClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-cm",
				Namespace: testNamespace,
				Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
			},
			Data: map[string]string{
				"admin.enabled": "true",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-secret",
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"admin.password":   []byte(hashed),
				"server.secretkey": []byte(testSecretKey),
			},
		},
	)
}

// newTestEnforcer creates an RBAC enforcer whose prevent-login-without-permissions
// flag and user policy are controlled by the caller.
func newTestEnforcer(t *testing.T, preventLogin bool, userPolicy string) *rbacpolicy.Enforcer {
	t.Helper()
	enf := utilrbac.NewEnforcer(fake.NewClientset(), testNamespace, testRBACCMName, nil)
	enf.SetPreventLoginWithoutPermissions(preventLogin)
	if userPolicy != "" {
		require.NoError(t, enf.SetUserPolicy(userPolicy))
	}
	return rbacpolicy.NewRBACPolicyEnforcer(enf, test.NewFakeProjLister())
}

func TestCreate_PreventLoginWithoutPermissions(t *testing.T) {
	tests := []struct {
		name         string
		preventLogin bool
		userPolicy   string
		wantCode     codes.Code
	}{
		{
			name:         "flag disabled, no permissions — login allowed",
			preventLogin: false,
			userPolicy:   "",
			wantCode:     codes.OK,
		},
		{
			name:         "flag enabled, no permissions — login blocked",
			preventLogin: true,
			userPolicy:   "",
			wantCode:     codes.PermissionDenied,
		},
		{
			name:         "flag enabled, user has permissions — login allowed",
			preventLogin: true,
			userPolicy:   "p, admin, applications, get, *, allow",
			wantCode:     codes.OK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeclientset := newTestKubeClient(t)
			settingsMgr := settings.NewSettingsManager(t.Context(), kubeclientset, testNamespace)
			redisClient, closer := test.NewInMemoryRedis()
			t.Cleanup(closer)
			mgr := sessionmgr.NewSessionManager(settingsMgr, test.NewFakeProjLister(), "", nil, sessionmgr.NewUserStateStorage(redisClient))
			policyEnf := newTestEnforcer(t, tt.preventLogin, tt.userPolicy)

			srv := NewServer(mgr, settingsMgr, nil, policyEnf, nil)
			_, err := srv.Create(t.Context(), &sessionpb.SessionCreateRequest{
				Username: "admin",
				Password: testAdminPass,
			})

			if tt.wantCode == codes.OK {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, status.Code(err))
			}
		})
	}
}

func TestGetUserInfo_PreventLoginWithoutPermissions(t *testing.T) {
	const ssoIssuer = "https://dex.example.com"

	tests := []struct {
		name         string
		loggedIn     bool
		iss          string
		preventLogin bool
		userPolicy   string
		username     string
		wantCode     codes.Code
	}{
		{
			name:         "not logged in — permission check skipped",
			loggedIn:     false,
			preventLogin: true,
			wantCode:     codes.OK,
		},
		{
			name:         "local user, flag disabled, no permissions — allowed",
			loggedIn:     true,
			iss:          sessionmgr.SessionManagerClaimsIssuer,
			preventLogin: false,
			username:     "alice",
			wantCode:     codes.OK,
		},
		{
			name:         "local user, flag enabled, no permissions — allowed (checked at Create instead)",
			loggedIn:     true,
			iss:          sessionmgr.SessionManagerClaimsIssuer,
			preventLogin: true,
			username:     "alice",
			wantCode:     codes.OK,
		},
		{
			name:         "SSO user, flag disabled, no permissions — allowed",
			loggedIn:     true,
			iss:          ssoIssuer,
			preventLogin: false,
			username:     "alice",
			wantCode:     codes.OK,
		},
		{
			name:         "SSO user, flag enabled, no permissions — blocked",
			loggedIn:     true,
			iss:          ssoIssuer,
			preventLogin: true,
			username:     "alice",
			wantCode:     codes.PermissionDenied,
		},
		{
			name:         "SSO user, flag enabled, has permissions — allowed",
			loggedIn:     true,
			iss:          ssoIssuer,
			preventLogin: true,
			userPolicy:   "p, alice, applications, get, *, allow",
			username:     "alice",
			wantCode:     codes.OK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policyEnf := newTestEnforcer(t, tt.preventLogin, tt.userPolicy)
			srv := NewServer(nil, nil, nil, policyEnf, nil)

			ctx := t.Context()
			if tt.loggedIn {
				// nolint:staticcheck // it's ok to use bulti-in type in a test
				ctx = context.WithValue(ctx, "claims", jwt.MapClaims{
					"sub": tt.username,
					"iss": tt.iss,
				})
			}

			resp, err := srv.GetUserInfo(ctx, &sessionpb.GetUserInfoRequest{})
			if tt.wantCode == codes.OK {
				require.NoError(t, err)
				assert.Equal(t, tt.loggedIn, resp.LoggedIn)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.wantCode, status.Code(err))
			}
		})
	}
}
