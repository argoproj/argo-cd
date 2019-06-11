package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
		"GIT_ASKPASS=git-ask-pass.sh",
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
	file, err := ioutil.TempFile("", "")
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
	strictHostKeyChecking := "yes"
	if c.insecureIgnoreHostKey {
		strictHostKeyChecking = "no"
	}
	return sshPrivateKeyFile(file.Name()),
		[]string{fmt.Sprintf("GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=%s -i %s", strictHostKeyChecking, file.Name())},
		nil
}
