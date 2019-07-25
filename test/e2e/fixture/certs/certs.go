package certs

import (
	"path/filepath"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
)

func AddCustomCACert() {
	caCertPath, err := filepath.Abs("../fixture/certs/argocd-test-ca.crt")
	errors.CheckError(err)
	args := []string{"cert", "add-tls", "localhost", "--from", caCertPath}
	errors.FailOnErr(fixture.RunCli(args...))
	args = []string{"cert", "add-tls", "127.0.0.1", "--from", caCertPath}
	errors.FailOnErr(fixture.RunCli(args...))
}
