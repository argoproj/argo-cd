package repos

import (
	"path/filepath"

	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/util/errors"
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

// AddSSHRepoCredentials adds E2E fixture credentials for SSH repos to context
func AddSSHCredentials() {
	keyPath, err := filepath.Abs("../fixture/testrepos/id_rsa")
	errors.CheckError(err)
	var repoURLType fixture.RepoURLType = fixture.RepoURLTypeSSH
	args := []string{"repocreds", "add", fixture.RepoBaseURL(repoURLType), "--ssh-private-key-path", keyPath}
	errors.FailOnErr(fixture.RunCli(args...))
}
