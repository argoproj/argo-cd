package application

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reconnect(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	ts := terminalSession{wsConn: c}
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
