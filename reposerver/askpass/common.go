package askpass

var (
	SocketPath = "/tmp/reposerver-ask-pass.sock"
)

type Creds struct {
	Username string
	Password string
}
