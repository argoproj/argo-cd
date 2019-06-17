package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/util"
)

type Creds interface {
	Environ() (io.Closer, []string, error)
}

// nop implementation
type NopCloser struct {
}

func (c NopCloser) Close() error {
	return nil
}

type NopCreds struct {
}

func (c NopCreds) Environ() (io.Closer, []string, error) {
	return NopCloser{}, nil, nil
}

// HTTPS creds implementation
type HTTPSCreds struct {
	username, password string
}

func NewHTTPSCreds(username, password string) HTTPSCreds {
	return HTTPSCreds{username, password}
}

func (c HTTPSCreds) Environ() (io.Closer, []string, error) {
	return NopCloser{}, []string{
		fmt.Sprintf("GIT_ASKPASS=%s", "git-ask-pass.sh"),
		fmt.Sprintf("GIT_USERNAME=%s", c.username),
		fmt.Sprintf("GIT_PASSWORD=%s", c.password),
	}, nil
}

// SSH implementation
type SSHCreds struct {
	sshPrivateKey         string
	insecureIgnoreHostKey bool
}

func NewSSHCreds(sshPrivateKey string, insecureIgnoreHostKey bool) SSHCreds {
	return SSHCreds{sshPrivateKey, insecureIgnoreHostKey}
}

type sshPrivateKeyFile string

func (f sshPrivateKeyFile) Close() error {
	return os.Remove(string(f))
}

func (c SSHCreds) Environ() (io.Closer, []string, error) {
	// use the SHM temp dir from util, more secure
	file, err := ioutil.TempFile(util.TempDir, "")
	if err != nil {
		return nil, nil, err
	}
	_, err = file.WriteString(c.sshPrivateKey)
	if err != nil {
		return nil, nil, err
	}
	err = file.Close()
	if err != nil {
		return nil, nil, err
	}
	args := []string{"ssh", "-i", file.Name()}
	if c.insecureIgnoreHostKey {
		log.Warn("temporarily disabling strict host key checking (i.e. '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'), please don't use in production")
		// StrictHostKeyChecking will add the host to the knownhosts file,  we don't want that - a security issue really,
		// UserKnownHostsFile=/dev/null is therefore used so we write the new insecure host to /dev/null
		args = append(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	}
	return sshPrivateKeyFile(file.Name()),
		[]string{fmt.Sprintf("GIT_SSH_COMMAND=%s", strings.Join(args, " "))},
		nil
}
