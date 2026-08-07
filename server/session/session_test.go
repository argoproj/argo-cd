package session

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	sessionmgr "github.com/argoproj/argo-cd/v3/util/session"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func newTestSessionServer(t *testing.T, cmData map[string]string) *Server {
	t.Helper()
	const ns = "default"
	kubeClient := fake.NewClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: ns,
				Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
			},
			Data: cmData,
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDSecretName,
				Namespace: ns,
			},
			Data: map[string][]byte{"server.secretkey": []byte("test")},
		},
	)
	settingsMgr := settings.NewSettingsManager(t.Context(), kubeClient, ns)
	enf := rbac.NewEnforcer(kubeClient, ns, common.ArgoCDConfigMapName, nil)
	policyEnf := rbacpolicy.NewRBACPolicyEnforcer(enf, nil)
	return NewServer(nil, settingsMgr, nil, policyEnf, nil)
}

func ctxWithClaims(claims jwt.MapClaims) context.Context {
	//nolint:staticcheck // the production code reads the "claims" string key from context
	return context.WithValue(context.Background(), "claims", claims)
}

func TestGetUserInfo_LocalUserStrictMode(t *testing.T) {
	localClaims := jwt.MapClaims{"sub": "sally", "iss": sessionmgr.SessionManagerClaimsIssuer}
	ssoClaims := jwt.MapClaims{"sub": "sally", "iss": "https://accounts.google.com", "email": "sally@example.com"}

	t.Run("strict mode appends @local for local accounts", func(t *testing.T) {
		s := newTestSessionServer(t, map[string]string{"rbac.local.user.strictmode": "true"})
		resp, err := s.GetUserInfo(ctxWithClaims(localClaims), nil)
		require.NoError(t, err)
		assert.Equal(t, "sally@local", resp.Username)
		assert.Equal(t, sessionmgr.SessionManagerClaimsIssuer, resp.Iss)
	})

	t.Run("strict mode does not affect SSO users", func(t *testing.T) {
		s := newTestSessionServer(t, map[string]string{"rbac.local.user.strictmode": "true"})
		resp, err := s.GetUserInfo(ctxWithClaims(ssoClaims), nil)
		require.NoError(t, err)
		assert.Equal(t, "sally@example.com", resp.Username)
	})

	t.Run("strict mode disabled leaves local username unchanged", func(t *testing.T) {
		s := newTestSessionServer(t, map[string]string{})
		resp, err := s.GetUserInfo(ctxWithClaims(localClaims), nil)
		require.NoError(t, err)
		assert.Equal(t, "sally", resp.Username)
	})
}
