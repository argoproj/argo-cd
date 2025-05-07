package certs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// Add a custom CA certificate to the test and also create the certificate file
// on the file system, so argocd-server and argocd-repo-server can use it.
func AddCustomCACert(t *testing.T) {
	t.Helper()
	caCertPath, err := filepath.Abs("../fixture/certs/argocd-test-ca.crt")
	require.NoError(t, err)
	// We need to setup TLS certs according to whether we are running tests
	// against a local workload (repositories available as localhost) and
	// against remote workloads (repositories available as argocd-e2e-server)
	if fixture.IsLocal() {
		args := []string{"cert", "add-tls", "localhost", "--from", caCertPath}
		errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
		args = []string{"cert", "add-tls", "127.0.0.1", "--from", caCertPath}
		errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
		certData, err := os.ReadFile(caCertPath)
		require.NoError(t, err)
		err = os.WriteFile(fixture.TmpDir+"/app/config/tls/localhost", certData, 0o644)
		require.NoError(t, err)
		err = os.WriteFile(fixture.TmpDir+"/app/config/tls/127.0.0.1", certData, 0o644)
		require.NoError(t, err)
	} else {
		args := []string{"cert", "add-tls", "argocd-e2e-server", "--from", caCertPath}
		errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
		fixture.RestartAPIServer(t)
		fixture.RestartRepoServer(t)
	}
}

// AddCustomSSHKnownHostsKeys adds SSH known hosts data to the Argo CD server
// being tested against. The env ARGOCD_E2E_SSH_KNOWN_HOSTS lets you specify
// an optional path to the known hosts file, instead of using the default one.
func AddCustomSSHKnownHostsKeys(t *testing.T) {
	t.Helper()
	source := os.Getenv("ARGOCD_E2E_SSH_KNOWN_HOSTS")
	if source == "" {
		source = "../fixture/testrepos/ssh_known_hosts"
	}
	knownHostsPath, err := filepath.Abs(source)
	require.NoError(t, err)
	args := []string{"cert", "add-ssh", "--upsert", "--batch", "--from", knownHostsPath}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))

	if fixture.IsLocal() {
		knownHostsData, err := os.ReadFile(knownHostsPath)
		require.NoError(t, err)
		err = os.WriteFile(fixture.TmpDir+"/app/config/ssh/ssh_known_hosts", knownHostsData, 0o644)
		require.NoError(t, err)
	} else {
		fixture.RestartAPIServer(t)
		fixture.RestartRepoServer(t)
	}
}
