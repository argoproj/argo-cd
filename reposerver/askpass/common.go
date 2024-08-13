package askpass

import (
	"github.com/argoproj/argo-cd/v2/util/env"
)

var SocketPath = "/tmp/reposerver-ask-pass.sock"

const (
	// ASKPASS_NONCE_ENV is the environment variable that is used to pass the nonce to the askpass script
	ASKPASS_NONCE_ENV = "ARGOCD_GIT_ASKPASS_NONCE"
	// AKSPASS_SOCKET_PATH_ENV is the environment variable that is used to pass the socket path to the askpass script
	AKSPASS_SOCKET_PATH_ENV = "ARGOCD_ASK_PASS_SOCK"
)

func init() {
	SocketPath = env.StringFromEnv(AKSPASS_SOCKET_PATH_ENV, SocketPath)
}

type Creds struct {
	Username string
	Password string
}
