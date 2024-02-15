package askpass

import (
	"github.com/argoproj/argo-cd/v2/util/env"
)

var (
	SocketPath = "/tmp/reposerver-ask-pass.sock"
)

func init() {
	SocketPath = env.StringFromEnv("ARGOCD_ASK_PASS_SOCK", SocketPath)
}

type Creds struct {
	Username string
	Password string
}
