package repos

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

var (
	CertPath    = mustToAbsPath("../fixture/certs/argocd-test-client.crt")
	CertKeyPath = mustToAbsPath("../fixture/certs/argocd-test-client.key")
)

func mustToAbsPath(relativePath string) string {
	res, err := filepath.Abs(relativePath)
	errors.CheckError(err)
	return res
}

// sets the current repo as the default SSH test repo
func AddSSHRepo(insecure bool, credentials bool, repoURLType fixture.RepoURLType) {
	keyPath, err := filepath.Abs("../fixture/testrepos/id_rsa")
	errors.CheckError(err)
	args := []string{"repo", "add", fixture.RepoURL(repoURLType)}
	if credentials {
		args = append(args, "--ssh-private-key-path", keyPath)
	}
	if insecure {
		args = append(args, "--insecure-ignore-host-key")
	}
	errors.FailOnErr(fixture.RunCli(args...))
}

// sets the current repo as the default HTTPS test repo
func AddHTTPSRepo(insecure bool, credentials bool, repoURLType fixture.RepoURLType) {
	// This construct is somewhat necessary to satisfy the compiler
	args := []string{"repo", "add", fixture.RepoURL(repoURLType)}
	if credentials {
		args = append(args, "--username", fixture.GitUsername, "--password", fixture.GitPassword)
	}
	if insecure {
		args = append(args, "--insecure-skip-server-verification")
	}
	errors.FailOnErr(fixture.RunCli(args...))
}

// sets a HTTPS repo using TLS client certificate authentication
func AddHTTPSRepoClientCert(insecure bool) {
	args := []string{
		"repo",
		"add",
		fixture.RepoURL(fixture.RepoURLTypeHTTPSClientCert),
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--tls-client-cert-path", CertPath,
		"--tls-client-cert-key-path", CertKeyPath,
	}
	if insecure {
		args = append(args, "--insecure-skip-server-verification")
	}
	errors.FailOnErr(fixture.RunCli(args...))
}

func AddHelmRepo(name string) {
	args := []string{
		"repo",
		"add",
		fixture.RepoURL(fixture.RepoURLTypeHelm),
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--tls-client-cert-path", CertPath,
		"--tls-client-cert-key-path", CertKeyPath,
		"--type", "helm",
		"--name", name,
	}
	errors.FailOnErr(fixture.RunCli(args...))
}

func AddHelmOCIRepo(name string) {
	args := []string{
		"repo",
		"add",
		fixture.HelmOCIRegistryURL,
		"--type", "helm",
		"--name", name,
		"--enable-oci",
	}
	errors.FailOnErr(fixture.RunCli(args...))
}

// AddHTTPSRepoCredentialsUserPass adds E2E username/password credentials for HTTPS repos to context
func AddHTTPSCredentialsUserPass() {
	var repoURLType fixture.RepoURLType = fixture.RepoURLTypeHTTPS
	args := []string{"repocreds", "add", fixture.RepoURL(repoURLType), "--username", fixture.GitUsername, "--password", fixture.GitPassword}
	errors.FailOnErr(fixture.RunCli(args...))
}

// AddHTTPSRepoCredentialsTLSClientCert adds E2E  for HTTPS repos to context
func AddHTTPSCredentialsTLSClientCert() {
	certPath, err := filepath.Abs("../fixture/certs/argocd-test-client.crt")
	errors.CheckError(err)
	keyPath, err := filepath.Abs("../fixture/certs/argocd-test-client.key")
	errors.CheckError(err)
	args := []string{
		"repocreds",
		"add",
		fixture.RepoBaseURL(fixture.RepoURLTypeHTTPSClientCert),
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--tls-client-cert-path", certPath,
		"--tls-client-cert-key-path", keyPath,
	}
	errors.FailOnErr(fixture.RunCli(args...))
}

// AddHelmHTTPSCredentialsTLSClientCert adds credentials for Helm repos to context
func AddHelmHTTPSCredentialsTLSClientCert() {
	certPath, err := filepath.Abs("../fixture/certs/argocd-test-client.crt")
	errors.CheckError(err)
	keyPath, err := filepath.Abs("../fixture/certs/argocd-test-client.key")
	errors.CheckError(err)
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
	errors.FailOnErr(fixture.RunCli(args...))
}

// AddHelmoOCICredentialsWithoutUserPass adds credentials for Helm OIC repo to context
func AddHelmoOCICredentialsWithoutUserPass() {
	args := []string{"repocreds", "add", fixture.RepoURL(fixture.RepoURLTypeHelmOCI),
		"--enable-oci", "--type", "helm"}
	errors.FailOnErr(fixture.RunCli(args...))
}

// AddSSHRepoCredentials adds E2E fixture credentials for SSH repos to context
func AddSSHCredentials() {
	keyPath, err := filepath.Abs("../fixture/testrepos/id_rsa")
	errors.CheckError(err)
	var repoURLType fixture.RepoURLType = fixture.RepoURLTypeSSH
	args := []string{"repocreds", "add", fixture.RepoBaseURL(repoURLType), "--ssh-private-key-path", keyPath}
	errors.FailOnErr(fixture.RunCli(args...))
}

// PushChartToOCIRegistry adds a helm chart to helm OCI registry
func PushChartToOCIRegistry(chartPathName, chartName, chartVersion string) {
	chartAbsPath, err := filepath.Abs(fmt.Sprintf("./testdata/%s", chartPathName))
	errors.CheckError(err)

	_ = os.Setenv("HELM_EXPERIMENTAL_OCI", "1")
	errors.FailOnErr(fixture.Run("", "helm", "chart", "save", chartAbsPath, fmt.Sprintf("%s/%s:%s", fixture.HelmOCIRegistryURL, chartName, chartVersion)))
	errors.FailOnErr(fixture.Run("", "helm", "chart", "push", fmt.Sprintf("%s/%s:%s", fixture.HelmOCIRegistryURL, chartName, chartVersion)))

}
