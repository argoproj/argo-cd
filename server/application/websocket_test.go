package application

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func reconnect(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{}
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
	assert.NoError(t, err)

	defer ws.Close()

	_, p, _ := ws.ReadMessage()

	var message TerminalMessage

	err = json.Unmarshal(p, &message)

	assert.NoError(t, err)
	assert.Equal(t, message.Data, ReconnectMessage)

}
