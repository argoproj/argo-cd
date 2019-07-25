package repos

import (
	"path/filepath"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
)

// sets the current repo as the default SSH test repo
func AddSSHRepo(insecure bool) {
	keyPath, err := filepath.Abs("../fixture/testrepos/id_rsa")
	errors.CheckError(err)
	args := []string{"repo", "add", fixture.RepoURL(fixture.RepoURLTypeSSH), "--ssh-private-key-path", keyPath}
	if insecure {
		args = append(args, "--insecure-ignore-host-key")
	}
	errors.FailOnErr(fixture.RunCli(args...))
}

// sets the current repo as the default HTTPS test repo
func AddHTTPSRepo(insecure bool) {
	// This construct is somewhat necessary to satisfy the compiler
	var repoURLType fixture.RepoURLType = fixture.RepoURLTypeHTTPS
	args := []string{"repo", "add", fixture.RepoURL(repoURLType), "--username", fixture.GitUsername, "--password", fixture.GitPassword}
	if insecure {
		args = append(args, "--insecure-skip-server-verification")
	}
	errors.FailOnErr(fixture.RunCli(args...))
}

// sets a HTTPS repo using TLS client certificate authentication
func AddHTTPSRepoClientCert(insecure bool) {
	certPath, err := filepath.Abs("../fixture/certs/argocd-test-client.crt")
	errors.CheckError(err)
	keyPath, err := filepath.Abs("../fixture/certs/argocd-test-client.key")
	errors.CheckError(err)
	args := []string{
		"repo",
		"add",
		fixture.RepoURL(fixture.RepoURLTypeHTTPSClientCert),
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--tls-client-cert-path", certPath,
		"--tls-client-cert-key-path", keyPath,
	}
	if insecure {
		args = append(args, "--insecure-skip-server-verification")
	}
	errors.FailOnErr(fixture.RunCli(args...))
}
