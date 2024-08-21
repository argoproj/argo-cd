package application

import (
	"context"
	"encoding/json"
	"k8s.io/client-go/tools/remotecommand"
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
	kubeclientset := fake.NewClientset(&v1.ConfigMap{
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

func TestTerminalSession_Read_Alternative(t *testing.T) {
	validate := func(w http.ResponseWriter, r *http.Request) {
		ts := newTestTerminalSession(w, r)
		if ts.wsConn == nil {
			t.Fatalf("WebSocket connection is not initialized")
		}

		ts.terminalOpts = &TerminalOptions{DisableAuth: true}
	}

	tests := []struct {
		name           string
		message        TerminalMessage
		expectedOutput string
		expectedSize   *remotecommand.TerminalSize
		expectedError  bool
	}{
		{
			name:           "stdin operation",
			message:        TerminalMessage{Operation: "stdin", Data: "test input"},
			expectedOutput: "test input",
			expectedSize:   nil,
			expectedError:  false,
		},
		{
			name:           "resize operation",
			message:        TerminalMessage{Operation: "resize", Cols: 80, Rows: 24},
			expectedOutput: "",
			expectedSize: &remotecommand.TerminalSize{
				Width:  80,
				Height: 24,
			},
			expectedError: false,
		},
		{
			name:           "unknown operation",
			message:        TerminalMessage{Operation: "unknown"},
			expectedOutput: EndOfTransmission,
			expectedSize:   nil,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(validate))
			defer s.Close()

			u := "ws" + strings.TrimPrefix(s.URL, "http")
			ws, _, err := websocket.DefaultDialer.Dial(u, nil)
			require.NoError(t, err)
			defer ws.Close()

			ts := terminalSession{
				wsConn:       ws,
				sizeChan:     make(chan remotecommand.TerminalSize, 1),
				terminalOpts: &TerminalOptions{DisableAuth: true},                                                            // Ensure terminalOpts is initialized
				token:        new(string),                                                                                    // Initialize token to avoid nil dereference
				ctx:          context.WithValue(context.Background(), "claims", &jwt.MapClaims{"groups": []string{"admin"}}), // Set context with claims
			}

			// Send the test message
			bytes, _ := json.Marshal(tt.message)
			err = ts.wsConn.WriteMessage(websocket.TextMessage, bytes)
			require.NoError(t, err)

			// Read data from the WebSocket
			p := make([]byte, 1024)
			n, err := ts.Read(p)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedOutput, string(p[:n]))
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOutput, string(p[:n]))
			}

			if tt.expectedSize != nil {
				select {
				case size := <-ts.sizeChan:
					assert.Equal(t, *tt.expectedSize, size)
				default:
					t.Error("Expected size message not received")
				}
			}
		})
	}
}

func TestVerifyAndReconnectDisableAuthTrue(t *testing.T) {
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
	validate := func(w http.ResponseWriter, r *http.Request) {
		enf := newEnforcer()
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
		enf.SetClaimsEnforcerFunc(func(claims jwt.Claims, rvals ...interface{}) bool {
			return true
		})
		ts := newTestTerminalSession(w, r)
		ts.terminalOpts = &TerminalOptions{Enf: enf}
		ts.appRBACName = "test"
		// nolint:staticcheck
		ts.ctx = context.WithValue(context.Background(), "claims", &jwt.MapClaims{"groups": []string{"admin"}})
		_, err := ts.validatePermissions([]byte{})
		require.NoError(t, err)
	}

	testServerConnection(t, validate, false)
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
		ts.terminalOpts = &TerminalOptions{Enf: enf}
		ts.appRBACName = "test"
		// nolint:staticcheck
		ts.ctx = context.WithValue(context.Background(), "claims", &jwt.MapClaims{"groups": []string{"test"}})
		_, err := ts.validatePermissions([]byte{})
		require.Error(t, err)
		assert.EqualError(t, err, permissionDeniedErr.Error())
	}

	testServerConnection(t, validate, true)
}

func TestTerminalSession_Read(t *testing.T) {
	validate := func(w http.ResponseWriter, r *http.Request) {
		ts := newTestTerminalSession(w, r)
		if ts.wsConn == nil {
			t.Fatalf("WebSocket connection is not initialized")
		}

		ts.terminalOpts = &TerminalOptions{DisableAuth: true}

		testMessages := []TerminalMessage{
			{
				Operation: "stdin",
				Data:      "test input",
			},
			{
				Operation: "resize",
				Cols:      80,
				Rows:      24,
			},
			{
				Operation: "unknown",
			},
		}

		for _, msg := range testMessages {
			bytes, _ := json.Marshal(msg)
			err := ts.wsConn.WriteMessage(websocket.TextMessage, bytes)
			if err != nil {
				t.Errorf("Failed to write message: %v", err)
			}
		}
	}

	tests := []struct {
		name           string
		expectedOutput string
		expectedSize   *remotecommand.TerminalSize
		expectedError  bool
	}{
		{
			name:           "stdin operation",
			expectedOutput: "test input",
			expectedSize:   nil,
			expectedError:  false,
		},
		{
			name:           "resize operation",
			expectedOutput: "", // Expecting no output for resize
			expectedSize: &remotecommand.TerminalSize{
				Width:  80,
				Height: 24,
			},
			expectedError: false,
		},
		{
			name:           "unknown operation",
			expectedOutput: EndOfTransmission,
			expectedSize:   nil,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(validate))
			defer s.Close()

			u := "ws" + strings.TrimPrefix(s.URL, "http")
			ws, _, err := websocket.DefaultDialer.Dial(u, nil)
			require.NoError(t, err)
			defer ws.Close()

			ts := terminalSession{
				wsConn:       ws,
				sizeChan:     make(chan remotecommand.TerminalSize, 1),
				terminalOpts: &TerminalOptions{DisableAuth: true},                                                            // Ensure terminalOpts is initialized
				token:        new(string),                                                                                    // Initialize token to avoid nil dereference
				ctx:          context.WithValue(context.Background(), "claims", &jwt.MapClaims{"groups": []string{"admin"}}), // Set context with claims
			}

			// Read data from the WebSocket
			p := make([]byte, 1024)
			n, err := ts.Read(p)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedOutput, string(p[:n]))
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOutput, string(p[:n]))
			}

			if tt.expectedSize != nil {
				select {
				case size := <-ts.sizeChan:
					assert.Equal(t, *tt.expectedSize, size)
				default:
					t.Error("Expected size message not received")
				}
			}
		})
	}
}

func TestTerminalSession_Write(t *testing.T) {
	validate := func(w http.ResponseWriter, r *http.Request) {
		ts := newTestTerminalSession(w, r)
		if ts.wsConn == nil {
			t.Fatalf("WebSocket connection is not initialized")
		}

		ts.terminalOpts = &TerminalOptions{DisableAuth: true}
	}

	s := httptest.NewServer(http.HandlerFunc(validate))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer ws.Close()

	ts := terminalSession{
		wsConn:   ws,
		sizeChan: make(chan remotecommand.TerminalSize, 1),
	}

	testData := "test output"

	n, err := ts.Write([]byte(testData))

	require.NoError(t, err)
	assert.Equal(t, len(testData), n)

	_, msg, err := ws.ReadMessage()
	require.NoError(t, err)

	var response TerminalMessage
	err = json.Unmarshal(msg, &response)
	require.NoError(t, err)

	assert.Equal(t, "stdout", response.Operation)
	assert.Equal(t, testData, response.Data)
}
