package certs

import (
	"io/ioutil"
	"path/filepath"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
)

// Add a custom CA certificate to the test and also create the certificate file
// on the file system, so argocd-server and argocd-repo-server can use it.
func AddCustomCACert() {
	caCertPath, err := filepath.Abs("../fixture/certs/argocd-test-ca.crt")
	errors.CheckError(err)
	args := []string{"cert", "add-tls", "localhost", "--from", caCertPath}
	errors.FailOnErr(fixture.RunCli(args...))
	args = []string{"cert", "add-tls", "127.0.0.1", "--from", caCertPath}
	errors.FailOnErr(fixture.RunCli(args...))

	certData, err := ioutil.ReadFile(caCertPath)
	errors.CheckError(err)
	err = ioutil.WriteFile(fixture.TmpDir+"/app/config/tls/localhost", certData, 0644)
	errors.CheckError(err)
	err = ioutil.WriteFile(fixture.TmpDir+"/app/config/tls/127.0.0.1", certData, 0644)
	errors.CheckError(err)
}

func AddCustomSSHKnownHostsKeys() {
	knownHostsPath, err := filepath.Abs("../fixture/certs/ssh_known_hosts")
	errors.CheckError(err)
	args := []string{"cert", "add-ssh", "--batch", "--from", knownHostsPath}
	errors.FailOnErr(fixture.RunCli(args...))

	knownHostsData, err := ioutil.ReadFile(knownHostsPath)
	errors.CheckError(err)
	err = ioutil.WriteFile(fixture.TmpDir+"/app/config/ssh/ssh_known_hosts", knownHostsData, 0644)
	errors.CheckError(err)
}
