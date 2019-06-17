package repos

import (
	"io/ioutil"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/test/fixture/testrepos"
)

// sets the current repo as the default SSH test repo
func AddSSHRepo() string {
	tmpFile, err := ioutil.TempFile("", "")
	errors.CheckError(err)
	r := testrepos.SSHTestRepo
	errors.FailOnErr(tmpFile.WriteString(r.SSHPrivateKey))
	errors.CheckError(tmpFile.Close())
	args := []string{"repo", "add", r.URL, "--ssh-private-key-path", tmpFile.Name()}
	if r.InsecureIgnoreHostKey {
		args = append(args, "--insecure-ignore-host-key")
	}
	errors.FailOnErr(fixture.RunCli(args...))
	return r.URL
}

// sets the current repo as the default HTTPS test repo
func AddHTTPSRepo() string {
	r := testrepos.HTTPSTestRepo
	errors.FailOnErr(fixture.RunCli("repo", "add", r.URL, "--username", r.Username, "--password", r.Password))
	return r.URL
}
