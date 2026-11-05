package session

import (
	"strconv"
	"testing"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/util/password"
	sessionmgr "github.com/argoproj/argo-cd/v3/util/session"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func newTestServer(t *testing.T, adminPassword string, adminEnabled bool) *Server {
	t.Helper()
	hash, err := password.HashPassword(adminPassword)
	require.NoError(t, err)
	kubeClient := fake.NewClientset(&corev1.ConfigMap{
		Name:      "argocd-cm",
		Namespace: "argocd",
		Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		Data: map[string]string{
			"admin":         string(settings.AccountCapabilityLogin),
			"admin.enabled": strconv.FormatBool(adminEnabled),
		},
	}, &corev1.Secret{
		Name:      "argocd-secret",
		Namespace: "argocd",
		Data: map[string][]byte{
			"admin.password":   []byte(hash),
			"server.secretkey": []byte("test-secret-key"),
		},
	})
	settingsMgr := settings.NewSettingsManager(t.Context(), kubeClient, "argocd")
	mgr := sessionmgr.NewSessionManager(settingsMgr, nil, "", nil, sessionmgr.NewUserStateStorage(nil))
	return NewServer(mgr, settingsMgr, nil, nil, nil)
}

func TestCreate_LogsLoginAttempt(t *testing.T) {
	const adminPassword = "password"

	for _, tc := range []struct {
		name          string
		adminEnabled  bool
		request       *session.SessionCreateRequest
		expectSuccess bool
	}{
		{
			name:          "successful login",
			adminEnabled:  true,
			request:       &session.SessionCreateRequest{Username: common.ArgoCDAdminUsername, Password: adminPassword},
			expectSuccess: true,
		},
		{
			name:         "wrong password",
			adminEnabled: true,
			request:      &session.SessionCreateRequest{Username: common.ArgoCDAdminUsername, Password: "wrong"},
		},
		{
			name:         "unknown user",
			adminEnabled: true,
			request:      &session.SessionCreateRequest{Username: "nobody", Password: adminPassword},
		},
		{
			name:         "disabled account",
			adminEnabled: false,
			request:      &session.SessionCreateRequest{Username: common.ArgoCDAdminUsername, Password: adminPassword},
		},
		{
			name:         "missing password",
			adminEnabled: true,
			request:      &session.SessionCreateRequest{Username: common.ArgoCDAdminUsername},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hook := logtest.NewGlobal()
			t.Cleanup(hook.Reset)
			s := newTestServer(t, adminPassword, tc.adminEnabled)

			resp, err := s.Create(t.Context(), tc.request)

			var entry *log.Entry
			for _, e := range hook.AllEntries() {
				if e.Data["login.type"] == "local" {
					entry = e
				}
			}
			require.NotNil(t, entry, "expected a login log entry")
			assert.Equal(t, tc.request.Username, entry.Data["username"])
			if tc.expectSuccess {
				require.NoError(t, err)
				assert.NotEmpty(t, resp.Token)
				assert.Equal(t, log.InfoLevel, entry.Level)
				assert.Equal(t, "Login successful", entry.Message)
				assert.NotContains(t, entry.Data, "error")
			} else {
				require.Error(t, err)
				assert.Equal(t, log.WarnLevel, entry.Level)
				assert.Equal(t, "Login failed", entry.Message)
				assert.Equal(t, err, entry.Data["error"])
			}
			for _, e := range hook.AllEntries() {
				assert.NotContains(t, e.Message, adminPassword)
				for _, v := range e.Data {
					assert.NotEqual(t, adminPassword, v)
				}
			}
		})
	}
}
