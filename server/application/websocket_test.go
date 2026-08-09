package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/assets"
	"github.com/argoproj/argo-cd/v3/util/rbac"

	"github.com/golang-jwt/jwt/v5"
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
	kubeclientset := fake.NewClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-cm",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: additionalConfig,
	}, &corev1.Secret{
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
	t.Parallel()
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

func testServerConnection(t *testing.T, testFunc func(w http.ResponseWriter, r *http.Request), expectPermissionDenied bool) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(testFunc))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http")

	// Connect to the server
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)

	defer ws.Close()
	if expectPermissionDenied {
		_, p, _ := ws.ReadMessage()

		var message TerminalMessage

		err = json.Unmarshal(p, &message)

		require.NoError(t, err)
		assert.Equal(t, "Permission denied", message.Data)
	}
}

func TestVerifyAndReconnectDisableAuthTrue(t *testing.T) {
	t.Parallel()
	validate := func(w http.ResponseWriter, r *http.Request) {
		ts := newTestTerminalSession(w, r)
		// Currently testing only the usecase of disableAuth: true since the disableAuth: false case
		// requires a valid token to be passed in the request.
		// Note that running with disableAuth: false will surprisingly succeed as well, because
		// the underlying token nil pointer dereference is swallowed in a location I didn't find,
		// or even swallowed by the test framework.
		ts.terminalOpts = &TerminalOptions{DisableAuth: true}
		code, err := ts.performValidationsAndReconnect([]byte{})
		assert.Equal(t, 0, code)
		require.NoError(t, err)
	}
	testServerConnection(t, validate, false)
}

func TestValidateWithAdminPermissions(t *testing.T) {
	t.Parallel()
	validate := func(w http.ResponseWriter, r *http.Request) {
		enf := newEnforcer()
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
		enf.SetClaimsEnforcerFunc(func(_ jwt.Claims, _ ...any) bool {
			return true
		})
		ts := newTestTerminalSession(w, r)
		ts.terminalOpts = &TerminalOptions{Enf: enf}
		ts.appRBACName = "test"
		//nolint:staticcheck
		ts.ctx = context.WithValue(t.Context(), "claims", &jwt.MapClaims{"groups": []string{"admin"}})
		_, err := ts.validatePermissions([]byte{})
		require.NoError(t, err)
	}

	testServerConnection(t, validate, false)
}

func TestValidateWithoutPermissions(t *testing.T) {
	t.Parallel()
	validate := func(w http.ResponseWriter, r *http.Request) {
		enf := newEnforcer()
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:test")
		enf.SetClaimsEnforcerFunc(func(_ jwt.Claims, _ ...any) bool {
			return false
		})
		ts := newTestTerminalSession(w, r)
		ts.terminalOpts = &TerminalOptions{Enf: enf}
		ts.appRBACName = "test"
		//nolint:staticcheck
		ts.ctx = context.WithValue(t.Context(), "claims", &jwt.MapClaims{"groups": []string{"test"}})
		_, err := ts.validatePermissions([]byte{})
		require.Error(t, err)
		assert.EqualError(t, err, common.PermissionDeniedAPIError.Error())
	}

	testServerConnection(t, validate, true)
}

func TestValidateActionUsesDebugValidator(t *testing.T) {
	t.Parallel()
	// A debug session sets permissionValidator; validateAction must consult it (and return its
	// result) instead of the terminal exec/create check, so a subject with only debug access
	// isn't denied mid-stream. The nil-validator (exec/create) path is covered by
	// TestValidateWithAdminPermissions / TestValidateWithoutPermissions.
	ts := &terminalSession{permissionValidator: func() error { return common.PermissionDeniedAPIError }}
	require.ErrorIs(t, ts.validateAction(), common.PermissionDeniedAPIError)

	ts.permissionValidator = func() error { return nil }
	require.NoError(t, ts.validateAction())
}

func TestTerminalSession_Write(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		for {
			// Read the message from the WebSocket connection
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			// Respond back the same message
			err = conn.WriteMessage(messageType, message)
			require.NoError(t, err)
		}
	}))
	defer server.Close()

	u := "ws" + strings.TrimPrefix(server.URL, "http")
	wsConn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer wsConn.Close()

	ts := terminalSession{
		wsConn: wsConn,
	}

	testData := []byte("hello world")
	expectedMessage, err := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      string(testData),
	})
	require.NoError(t, err)

	n, err := ts.Write(testData)
	require.NoError(t, err)

	assert.Equal(t, len(testData), n)

	_, receivedMessage, err := wsConn.ReadMessage()
	require.NoError(t, err)

	assert.Equal(t, expectedMessage, receivedMessage)
}

func TestGetToken(t *testing.T) {
	t.Parallel()
	// jwtutil.IsValid only checks JWT shape (three dot-separated segments).
	const tokenValue = "header.payload.signature"
	const cookieToken = "cookie.payload.signature"

	t.Run("bearer token in Authorization header", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/terminal", http.NoBody)
		r.Header.Set("Authorization", "Bearer "+tokenValue)
		token, err := getToken(r)
		require.NoError(t, err)
		assert.Equal(t, tokenValue, token)
	})

	t.Run("auth cookie when no Authorization header", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/terminal", http.NoBody)
		r.AddCookie(&http.Cookie{Name: common.AuthCookieName, Value: tokenValue})
		token, err := getToken(r)
		require.NoError(t, err)
		assert.Equal(t, tokenValue, token)
	})

	t.Run("bearer token preferred over cookie", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/terminal", http.NoBody)
		r.Header.Set("Authorization", "Bearer "+tokenValue)
		r.AddCookie(&http.Cookie{Name: common.AuthCookieName, Value: cookieToken})
		token, err := getToken(r)
		require.NoError(t, err)
		assert.Equal(t, tokenValue, token)
	})

	t.Run("invalid bearer falls back to valid cookie", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/terminal", http.NoBody)
		r.Header.Set("Authorization", "Bearer not-a-jwt")
		r.AddCookie(&http.Cookie{Name: common.AuthCookieName, Value: cookieToken})
		token, err := getToken(r)
		require.NoError(t, err)
		assert.Equal(t, cookieToken, token)
	})

	t.Run("rejects invalid bearer without cookie", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/terminal", http.NoBody)
		r.Header.Set("Authorization", "Bearer not-a-jwt")
		_, err := getToken(r)
		require.Error(t, err)
	})

	t.Run("rejects invalid cookie token", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/terminal", http.NoBody)
		r.AddCookie(&http.Cookie{Name: common.AuthCookieName, Value: "not-a-jwt"})
		_, err := getToken(r)
		require.Error(t, err)
	})
}
