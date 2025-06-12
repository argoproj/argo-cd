package repos

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

func CertPath(t *testing.T) string {
	t.Helper()
	return mustToAbsPath(t, "../fixture/certs/argocd-test-client.crt")
}

func CertKeyPath(t *testing.T) string {
	t.Helper()
	return mustToAbsPath(t, "../fixture/certs/argocd-test-client.key")
}

func mustToAbsPath(t *testing.T, relativePath string) string {
	t.Helper()
	res, err := filepath.Abs(relativePath)
	require.NoError(t, err)
	return res
}

// sets the current repo as the default SSH test repo
func AddSSHRepo(t *testing.T, insecure bool, credentials bool, repoURLType fixture.RepoURLType) {
	t.Helper()
	keyPath, err := filepath.Abs("../fixture/testrepos/id_rsa")
	require.NoError(t, err)
	args := []string{"repo", "add", fixture.RepoURL(repoURLType)}
	if credentials {
		args = append(args, "--ssh-private-key-path", keyPath)
	}
	if insecure {
		args = append(args, "--insecure-ignore-host-key")
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

// sets the current repo as the default HTTPS test repo
func AddHTTPSRepo(t *testing.T, insecure bool, credentials bool, project string, repoURLType fixture.RepoURLType) {
	t.Helper()
	// This construct is somewhat necessary to satisfy the compiler
	args := []string{"repo", "add", fixture.RepoURL(repoURLType)}
	if credentials {
		args = append(args, "--username", fixture.GitUsername, "--password", fixture.GitPassword)
	}
	if insecure {
		args = append(args, "--insecure-skip-server-verification")
	}
	if project != "" {
		args = append(args, "--project", project)
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

// sets a HTTPS repo using TLS client certificate authentication
func AddHTTPSRepoClientCert(t *testing.T, insecure bool) {
	t.Helper()
	args := []string{
		"repo",
		"add",
		fixture.RepoURL(fixture.RepoURLTypeHTTPSClientCert),
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--tls-client-cert-path", CertPath(t),
		"--tls-client-cert-key-path", CertKeyPath(t),
	}
	if insecure {
		args = append(args, "--insecure-skip-server-verification")
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

func AddHelmRepo(t *testing.T, name string) {
	t.Helper()
	args := []string{
		"repo",
		"add",
		fixture.RepoURL(fixture.RepoURLTypeHelm),
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--tls-client-cert-path", CertPath(t),
		"--tls-client-cert-key-path", CertKeyPath(t),
		"--type", "helm",
		"--name", name,
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

func AddHelmOCIRepo(t *testing.T, name string) {
	t.Helper()
	args := []string{
		"repo",
		"add",
		fixture.HelmOCIRegistryURL,
		"--type", "helm",
		"--name", name,
		"--enable-oci",
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

// AddHTTPSRepoCredentialsUserPass adds E2E username/password credentials for HTTPS repos to context
func AddHTTPSCredentialsUserPass(t *testing.T) {
	t.Helper()
	var repoURLType fixture.RepoURLType = fixture.RepoURLTypeHTTPS
	args := []string{"repocreds", "add", fixture.RepoURL(repoURLType), "--username", fixture.GitUsername, "--password", fixture.GitPassword}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

// AddHTTPSRepoCredentialsTLSClientCert adds E2E  for HTTPS repos to context
func AddHTTPSCredentialsTLSClientCert(t *testing.T) {
	t.Helper()
	certPath, err := filepath.Abs("../fixture/certs/argocd-test-client.crt")
	require.NoError(t, err)
	keyPath, err := filepath.Abs("../fixture/certs/argocd-test-client.key")
	require.NoError(t, err)
	args := []string{
		"repocreds",
		"add",
		fixture.RepoBaseURL(fixture.RepoURLTypeHTTPSClientCert),
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--tls-client-cert-path", certPath,
		"--tls-client-cert-key-path", keyPath,
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

// AddHelmHTTPSCredentialsTLSClientCert adds credentials for Helm repos to context
func AddHelmHTTPSCredentialsTLSClientCert(t *testing.T) {
	t.Helper()
	certPath, err := filepath.Abs("../fixture/certs/argocd-test-client.crt")
	require.NoError(t, err)
	keyPath, err := filepath.Abs("../fixture/certs/argocd-test-client.key")
	require.NoError(t, err)
	args := []string{
		"repocreds",
		"add",
		fixture.RepoURL(fixture.RepoURLTypeHelmParent),
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--tls-client-cert-path", certPath,
		"--tls-client-cert-key-path", keyPath,
		"--type", "helm",
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

// AddHelmoOCICredentialsWithoutUserPass adds credentials for Helm OIC repo to context
func AddHelmoOCICredentialsWithoutUserPass(t *testing.T) {
	t.Helper()
	args := []string{
		"repocreds", "add", fixture.RepoURL(fixture.RepoURLTypeHelmOCI),
		"--enable-oci", "--type", "helm",
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

// AddSSHRepoCredentials adds E2E fixture credentials for SSH repos to context
func AddSSHCredentials(t *testing.T) {
	t.Helper()
	keyPath, err := filepath.Abs("../fixture/testrepos/id_rsa")
	require.NoError(t, err)
	var repoURLType fixture.RepoURLType = fixture.RepoURLTypeSSH
	args := []string{"repocreds", "add", fixture.RepoBaseURL(repoURLType), "--ssh-private-key-path", keyPath}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

// PushChartToOCIRegistry adds a helm chart to helm OCI registry
func PushChartToOCIRegistry(t *testing.T, chartPathName, chartName, chartVersion string) {
	t.Helper()
	// create empty temp directory to extract chart from the registry
	tempDest, err1 := os.MkdirTemp("", "helm")
	require.NoError(t, err1)
	defer func() { _ = os.RemoveAll(tempDest) }()

	chartAbsPath, err2 := filepath.Abs("./testdata/" + chartPathName)
	require.NoError(t, err2)

	t.Setenv("HELM_EXPERIMENTAL_OCI", "1")
	errors.NewHandler(t).FailOnErr(fixture.Run("", "helm", "dependency", "build", chartAbsPath))
	errors.NewHandler(t).FailOnErr(fixture.Run("", "helm", "package", chartAbsPath, "--destination", tempDest))
	_ = os.RemoveAll(fmt.Sprintf("%s/%s", chartAbsPath, "charts"))
	errors.NewHandler(t).FailOnErr(fixture.Run(
		"",
		"helm",
		"push",
		fmt.Sprintf("%s/%s-%s.tgz", tempDest, chartName, chartVersion),
		"oci://"+fixture.HelmOCIRegistryURL,
	))
}
