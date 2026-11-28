package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
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
		Namespace: testNamespace,
		Name:      "argocd-cm",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: additionalConfig,
	}, &corev1.Secret{
		Name:      "argocd-secret",
		Namespace: testNamespace,
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

func newTerminalWebSocketConnection(t *testing.T) *websocket.Conn {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
	}))
	t.Cleanup(server.Close)

	wsConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = wsConn.Close() })
	return wsConn
}

func TestNewTerminalSessionCloseCancelsContext(t *testing.T) {
	t.Parallel()
	sessions := make(chan *terminalSession, 1)
	errs := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(&http.Cookie{Name: common.AuthCookieName, Value: "header.payload.signature"})
		session, err := newTerminalSession(r.Context(), w, r, nil, nil, "", nil)
		if err != nil {
			errs <- err
			return
		}
		sessions <- session
	}))
	defer server.Close()

	wsConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer wsConn.Close()

	var session *terminalSession
	select {
	case session = <-sessions:
	case err := <-errs:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("terminal session was not created")
	}
	require.NoError(t, session.Close())
	assert.ErrorIs(t, session.ctx.Err(), context.Canceled)
}

func TestTerminalSessionErrorsCancelContext(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name string
		fail func(*terminalSession) error
	}{
		{
			name: "read",
			fail: func(session *terminalSession) error {
				_, err := session.Read(make([]byte, 1))
				return err
			},
		},
		{name: "ping", fail: (*terminalSession).Ping},
		{
			name: "write",
			fail: func(session *terminalSession) error {
				_, err := session.Write([]byte("output"))
				return err
			},
		},
		{
			name: "reconnect",
			fail: func(session *terminalSession) error {
				_, err := session.reconnect()
				return err
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			wsConn := newTerminalWebSocketConnection(t)
			require.NoError(t, wsConn.Close())

			session := terminalSession{
				ctx:          ctx,
				cancel:       cancel,
				wsConn:       wsConn,
				terminalOpts: &TerminalOptions{DisableAuth: true},
			}
			require.Error(t, testCase.fail(&session))
			assert.ErrorIs(t, ctx.Err(), context.Canceled)
		})
	}
}

func TestTerminalSessionDoneCancelsContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	session := terminalSession{
		ctx:      ctx,
		cancel:   cancel,
		doneChan: make(chan struct{}),
	}

	session.Done()

	assert.ErrorIs(t, ctx.Err(), context.Canceled)
	select {
	case <-session.doneChan:
	default:
		t.Fatal("done channel was not closed")
	}
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

func TestReconnectReleasesWriteLockOnError(t *testing.T) {
	t.Parallel()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
	}))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http")
	wsConn, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	require.NoError(t, wsConn.Close())

	ts := terminalSession{wsConn: wsConn}
	_, err = ts.reconnect()
	require.Error(t, err)

	// A leaked lock would block every later Write and Ping on this session.
	require.True(t, ts.writeLock.TryLock())
	ts.writeLock.Unlock()
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
