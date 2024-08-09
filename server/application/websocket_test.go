package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/argoproj/argo-cd/v2/util/rbac"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTerminalSession(w http.ResponseWriter, r *http.Request) terminalSession {
	upgrader := websocket.Upgrader{}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return terminalSession{}
	}

	return terminalSession{wsConn: c}
}

func newEnforcer() *rbac.Enforcer {
	additionalConfig := make(map[string]string, 0)
	kubeclientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-cm",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: additionalConfig,
	}, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})

	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	return enforcer
}

func reconnect(w http.ResponseWriter, r *http.Request) {
	ts := newTestTerminalSession(w, r)
	_, _ = ts.reconnect()
}

func TestReconnect(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(reconnect))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http")

	// Connect to the server
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	defer ws.Close()

	_, p, _ := ws.ReadMessage()

	var message TerminalMessage

	err = json.Unmarshal(p, &message)

	require.NoError(t, err)
	assert.Equal(t, ReconnectMessage, message.Data)
}

func TestValidateWithAdminPermissions(t *testing.T) {
	validate := func(w http.ResponseWriter, r *http.Request) {
		enf := newEnforcer()
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
		enf.SetClaimsEnforcerFunc(func(claims jwt.Claims, rvals ...interface{}) bool {
			return true
		})
		ts := newTestTerminalSession(w, r)
		ts.enf = enf
		ts.appRBACName = "test"
		// nolint:staticcheck
		ts.ctx = context.WithValue(context.Background(), "claims", &jwt.MapClaims{"groups": []string{"admin"}})
		_, err := ts.validatePermissions([]byte{})
		require.NoError(t, err)
	}

	s := httptest.NewServer(http.HandlerFunc(validate))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http")

	// Connect to the server
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	defer ws.Close()
}

func TestValidateWithoutPermissions(t *testing.T) {
	validate := func(w http.ResponseWriter, r *http.Request) {
		enf := newEnforcer()
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:test")
		enf.SetClaimsEnforcerFunc(func(claims jwt.Claims, rvals ...interface{}) bool {
			return false
		})
		ts := newTestTerminalSession(w, r)
		ts.enf = enf
		ts.appRBACName = "test"
		// nolint:staticcheck
		ts.ctx = context.WithValue(context.Background(), "claims", &jwt.MapClaims{"groups": []string{"test"}})
		_, err := ts.validatePermissions([]byte{})
		require.Error(t, err)
		assert.Equal(t, permissionDeniedErr.Error(), err.Error())
	}

	s := httptest.NewServer(http.HandlerFunc(validate))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http")

	// Connect to the server
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	defer ws.Close()

	_, p, _ := ws.ReadMessage()

	var message TerminalMessage

	err = json.Unmarshal(p, &message)

	require.NoError(t, err)
	assert.Equal(t, "Permission denied", message.Data)
}
