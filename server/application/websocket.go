package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	httputil "github.com/argoproj/argo-cd/v2/util/http"
	util_session "github.com/argoproj/argo-cd/v2/util/session"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	ReconnectCode    = 1
	ReconnectMessage = "\nReconnect because the token was refreshed...\n"
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
	ctx            context.Context
	wsConn         *websocket.Conn
	sizeChan       chan remotecommand.TerminalSize
	doneChan       chan struct{}
	tty            bool
	readLock       sync.Mutex
	writeLock      sync.Mutex
	sessionManager *util_session.SessionManager
	token          *string
	appRBACName    string
	terminalOpts   *TerminalOptions
}

// getToken get auth token from web socket request
func getToken(r *http.Request) (string, error) {
	cookies := r.Cookies()
	return httputil.JoinCookies(common.AuthCookieName, cookies)
}

// newTerminalSession create terminalSession
func newTerminalSession(ctx context.Context, w http.ResponseWriter, r *http.Request, responseHeader http.Header, sessionManager *util_session.SessionManager, appRBACName string, terminalOpts *TerminalOptions) (*terminalSession, error) {
	token, err := getToken(r)
	if err != nil {
		return nil, err
	}

	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return nil, err
	}
	session := &terminalSession{
		ctx:            ctx,
		wsConn:         conn,
		tty:            true,
		sizeChan:       make(chan remotecommand.TerminalSize),
		doneChan:       make(chan struct{}),
		sessionManager: sessionManager,
		token:          &token,
		appRBACName:    appRBACName,
		terminalOpts:   terminalOpts,
	}
	return session, nil
}

// Done close the done channel.
func (t *terminalSession) Done() {
	close(t.doneChan)
}

func (t *terminalSession) StartKeepalives(dur time.Duration) {
	ticker := time.NewTicker(dur)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := t.Ping()
			if err != nil {
				log.Errorf("ping error: %v", err)
				return
			}
		case <-t.doneChan:
			return
		}
	}
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

// reconnect send reconnect code to client and ask them init new ws session
func (t *terminalSession) reconnect() (int, error) {
	reconnectCommand, _ := json.Marshal(TerminalCommand{
		Code: ReconnectCode,
	})
	reconnectMessage, _ := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      ReconnectMessage,
	})
	t.writeLock.Lock()
	err := t.wsConn.WriteMessage(websocket.TextMessage, reconnectMessage)
	if err != nil {
		log.Errorf("write message err: %v", err)
		return 0, err
	}
	err = t.wsConn.WriteMessage(websocket.TextMessage, reconnectCommand)
	if err != nil {
		log.Errorf("write message err: %v", err)
		return 0, err
	}
	t.writeLock.Unlock()
	return 0, nil
}

func (t *terminalSession) validatePermissions(p []byte) (int, error) {
	permissionDeniedMessage, _ := json.Marshal(TerminalMessage{
		Operation: "stdout",
		Data:      "Permission denied",
	})
	if err := t.terminalOpts.Enf.EnforceErr(t.ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, t.appRBACName); err != nil {
		err = t.wsConn.WriteMessage(websocket.TextMessage, permissionDeniedMessage)
		if err != nil {
			log.Errorf("permission denied message err: %v", err)
		}
		return copy(p, EndOfTransmission), permissionDeniedErr
	}

	if err := t.terminalOpts.Enf.EnforceErr(t.ctx.Value("claims"), rbacpolicy.ResourceExec, rbacpolicy.ActionCreate, t.appRBACName); err != nil {
		err = t.wsConn.WriteMessage(websocket.TextMessage, permissionDeniedMessage)
		if err != nil {
			log.Errorf("permission denied message err: %v", err)
		}
		return copy(p, EndOfTransmission), permissionDeniedErr
	}
	return 0, nil
}

func (t *terminalSession) performValidationsAndReconnect(p []byte) (int, error) {
	// In disable auth mode, no point verifying the token or validating permissions
	if t.terminalOpts.DisableAuth {
		return 0, nil
	}

	// check if token still valid
	_, newToken, err := t.sessionManager.VerifyToken(*t.token)
	// err in case if token is revoked, newToken in case if refresh happened
	if err != nil || newToken != "" {
		// need to send reconnect code in case if token was refreshed
		return t.reconnect()
	}
	code, err := t.validatePermissions(p)
	if err != nil {
		return code, err
	}

	return 0, nil
}

// Read called in a loop from remotecommand as long as the process is running
func (t *terminalSession) Read(p []byte) (int, error) {
	code, err := t.performValidationsAndReconnect(p)
	if err != nil {
		return code, err
	}

	t.readLock.Lock()
	_, message, err := t.wsConn.ReadMessage()
	t.readLock.Unlock()
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

// Ping called periodically to ensure connection stays alive through load balancers
func (t *terminalSession) Ping() error {
	t.writeLock.Lock()
	err := t.wsConn.WriteMessage(websocket.PingMessage, []byte("ping"))
	t.writeLock.Unlock()
	if err != nil {
		log.Errorf("ping message err: %v", err)
	}
	return err
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
	t.writeLock.Lock()
	err = t.wsConn.WriteMessage(websocket.TextMessage, msg)
	t.writeLock.Unlock()
	if err != nil {
		log.Errorf("write message err: %v", err)
		return 0, err
	}
	return len(p), nil
}

// Close closes websocket connection
func (t *terminalSession) Close() error {
	return t.wsConn.Close()
}
