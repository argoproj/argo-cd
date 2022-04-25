package application

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

var upgrader = func() websocket.Upgrader {
	upgrader := websocket.Upgrader{}
	upgrader.HandshakeTimeout = time.Second * 2
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	return upgrader
}()

// terminalSession implements PtyHandler
type terminalSession struct {
	wsConn   *websocket.Conn
	sizeChan chan remotecommand.TerminalSize
	doneChan chan struct{}
	tty      bool
}

// newTerminalSession create terminalSession
func newTerminalSession(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*terminalSession, error) {
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}
	session := &terminalSession{
		wsConn:   conn,
		tty:      true,
		sizeChan: make(chan remotecommand.TerminalSize),
		doneChan: make(chan struct{}),
	}
	return session, nil
}

// Done close the done channel.
func (t *terminalSession) Done() {
	close(t.doneChan)
}

// Next called in a loop from remotecommand as long as the process is running
func (t *terminalSession) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.sizeChan:
		return &size
	case <-t.doneChan:
		return nil
	}
}

// Read called in a loop from remotecommand as long as the process is running
func (t *terminalSession) Read(p []byte) (int, error) {
	_, message, err := t.wsConn.ReadMessage()
	if err != nil {
		log.Errorf("read message err: %v", err)
		return copy(p, EndOfTransmission), err
	}
	var msg TerminalMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Errorf("read parse message err: %v", err)
		return copy(p, EndOfTransmission), err
	}
	switch msg.Operation {
	case "stdin":
		return copy(p, msg.Data), nil
	case "resize":
		t.sizeChan <- remotecommand.TerminalSize{Width: msg.Cols, Height: msg.Rows}
		return 0, nil
	default:
		return copy(p, EndOfTransmission), fmt.Errorf("unknown message type %s", msg.Operation)
	}
}

// Write called from remotecommand whenever there is any output
func (t *terminalSession) Write(p []byte) (int, error) {
	msg, err := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      string(p),
	})
	if err != nil {
		log.Errorf("write parse message err: %v", err)
		return 0, err
	}
	if err := t.wsConn.WriteMessage(websocket.TextMessage, msg); err != nil {
		log.Errorf("write message err: %v", err)
		return 0, err
	}
	return len(p), nil
}

// Close closes websocket connection
func (t *terminalSession) Close() error {
	return t.wsConn.Close()
}
