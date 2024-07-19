package askpass

import (
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/git"
)

var SocketPath = "/tmp/reposerver-ask-pass.sock"

const CommitServerSocketPath = "/tmp/commit-server-ask-pass.sock"

func init() {
	SocketPath = env.StringFromEnv(git.AKSPASS_SOCKET_PATH_ENV, SocketPath)
}

type Creds struct {
	Username string
	Password string
}
