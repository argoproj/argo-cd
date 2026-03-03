package repos

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/common"
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

type AddRepoOpts func(args []string) []string

func WithDepth(depth int64) AddRepoOpts {
	return func(args []string) []string {
		if depth > 0 {
			args = append(args, "--depth", strconv.FormatInt(depth, 10))
		}
		return args
	}
}

func applyOpts(args []string, opts []AddRepoOpts) []string {
	for _, opt := range opts {
		args = opt(args)
	}
	return args
}

// sets the current repo as the default SSH test repo
func AddSSHRepo(t *testing.T, insecure bool, credentials bool, repoURLType fixture.RepoURLType, opts ...AddRepoOpts) {
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
	errors.NewHandler(t).FailOnErr(fixture.RunCli(applyOpts(args, opts)...))
}

// sets the current repo as the default HTTPS test repo
func AddHTTPSRepo(t *testing.T, insecure bool, credentials bool, project string, repoURLType fixture.RepoURLType, opts ...AddRepoOpts) {
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
	errors.NewHandler(t).FailOnErr(fixture.RunCli(applyOpts(args, opts)...))
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

// AddHelmProvenanceRepo adds the local helm-repo/provenance sub-repo with signed charts.
func AddHelmProvenanceRepo(t *testing.T, name string) {
	t.Helper()
	repoURL := strings.TrimSuffix(fixture.RepoURL(fixture.RepoURLTypeHelmParent), "/") + "/provenance"
	args := []string{
		"repo",
		"add",
		repoURL,
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--type", "helm",
		"--name", name,
	}
	if strings.HasPrefix(repoURL, "https://") {
		args = append(args, "--tls-client-cert-path", CertPath(t), "--tls-client-cert-key-path", CertKeyPath(t))
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

func AddOCIRepo(t *testing.T, name, imagePath string) {
	t.Helper()
	args := []string{
		"repo",
		"add",
		fmt.Sprintf("%s/%s", fixture.OCIHostURL, imagePath),
		"--type", "oci",
		"--name", name,
		"--insecure-oci-force-http",
	}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
}

func AddAuthenticatedOCIRepo(t *testing.T, name, imagePath string) {
	t.Helper()
	args := []string{
		"repo",
		"add",
		fmt.Sprintf("%s/%s", fixture.AuthenticatedOCIHostURL, imagePath),
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"--type", "oci",
		"--name", name,
		"--insecure-oci-force-http",
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
		"--insecure-oci-force-http",
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

// PushChartWithProvenanceToOCIRegistry packages a chart with provenance (signed) and pushes to OCI.
// Pushes to HelmOCIRegistryURL (localhost:5000/myrepo). Helm push includes .prov when
// it is co-located with the .tgz. Requires test/fixture/gpg/signingkey.asc for signing.
func PushChartWithProvenanceToOCIRegistry(t *testing.T, chartPathName, chartName, chartVersion string) {
	t.Helper()
	tempDest, err := os.MkdirTemp("", "helm-prov")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDest) }()

	chartAbsPath, err := filepath.Abs("./" + chartPathName)
	require.NoError(t, err)
	signingKey, err := filepath.Abs("../fixture/gpg/signingkey.asc")
	require.NoError(t, err)

	// 1. Package (unsigned)
	errors.NewHandler(t).FailOnErr(fixture.Run("", "helm", "package", chartAbsPath, "--destination", tempDest))
	tgzPath := filepath.Join(tempDest, fmt.Sprintf("%s-%s.tgz", chartName, chartVersion))
	tgzData, err := os.ReadFile(tgzPath)
	require.NoError(t, err)

	// 2. Compute SHA256
	digest := sha256.Sum256(tgzData)
	digestStr := hex.EncodeToString(digest[:])

	// 3. Build provenance body
	chartYaml, err := os.ReadFile(filepath.Join(chartAbsPath, "Chart.yaml"))
	require.NoError(t, err)
	provBody := string(chartYaml) + fmt.Sprintf("\nfiles:\n  %s-%s.tgz: sha256:%s\n", chartName, chartVersion, digestStr)

	// 4. Sign with gpg (use temp GNUPGHOME)
	gnupgHome := filepath.Join(tempDest, "gnupg")
	require.NoError(t, os.MkdirAll(gnupgHome, 0o700))
	provBodyPath := filepath.Join(tempDest, "prov-body.txt")
	require.NoError(t, os.WriteFile(provBodyPath, []byte(provBody), 0o600))
	provPath := tgzPath + ".prov"
	errors.NewHandler(t).FailOnErr(fixture.Run(tempDest, "env", "GNUPGHOME="+gnupgHome, "gpg", "--batch", "--import", signingKey))
	errors.NewHandler(t).FailOnErr(fixture.Run(tempDest, "env", "GNUPGHOME="+gnupgHome,
		"gpg", "--batch", "--yes", "--local-user", fixture.GpgGoodKeyID,
		"--clearsign", "--output", provPath, provBodyPath))

	// 5. Helm push (includes .prov when present)
	errors.NewHandler(t).FailOnErr(fixture.Run(tempDest, "helm", "push",
		fmt.Sprintf("%s-%s.tgz", chartName, chartVersion),
		"oci://"+fixture.HelmOCIRegistryURL))
}

// PushChartToOCIRegistry adds a helm chart to helm OCI registry
func PushChartToOCIRegistry(t *testing.T, chartPathName, chartName, chartVersion string) {
	t.Helper()
	// create empty temp directory to extract chart from the registry
	tempDest, err1 := os.MkdirTemp("", "helm")
	require.NoError(t, err1)
	defer func() { _ = os.RemoveAll(tempDest) }()

	chartAbsPath, err2 := filepath.Abs("./" + chartPathName)
	require.NoError(t, err2)

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

// PushChartToAuthenticatedOCIRegistry adds a helm chart to helm OCI registry
func PushChartToAuthenticatedOCIRegistry(t *testing.T, chartPathName, chartName, chartVersion string) {
	t.Helper()
	// create empty temp directory to extract chart from the registry
	tempDest, err1 := os.MkdirTemp("", "helm")
	require.NoError(t, err1)
	defer func() { _ = os.RemoveAll(tempDest) }()

	chartAbsPath, err2 := filepath.Abs("./" + chartPathName)
	require.NoError(t, err2)

	errors.NewHandler(t).FailOnErr(fixture.Run("", "helm", "dependency", "build", chartAbsPath))
	errors.NewHandler(t).FailOnErr(fixture.Run("", "helm", "package", chartAbsPath, "--destination", tempDest))
	_ = os.RemoveAll(fmt.Sprintf("%s/%s", chartAbsPath, "charts"))

	errors.NewHandler(t).FailOnErr(fixture.Run(
		"",
		"helm",
		"registry",
		"login",
		"--username", fixture.GitUsername,
		"--password", fixture.GitPassword,
		"localhost:5001",
	))

	errors.NewHandler(t).FailOnErr(fixture.Run(
		"",
		"helm",
		"push",
		fmt.Sprintf("%s/%s-%s.tgz", tempDest, chartName, chartVersion),
		"oci://"+fixture.HelmAuthenticatedOCIRegistryURL,
	))

	errors.NewHandler(t).FailOnErr(fixture.Run(
		"",
		"helm",
		"registry",
		"logout",
		"localhost:5001",
	))
}

// PushImageToOCIRegistry adds a helm chart to helm OCI registry
func PushImageToOCIRegistry(t *testing.T, pathName, tag string) {
	t.Helper()
	imagePath := "./" + pathName

	errors.NewHandler(t).FailOnErr(fixture.Run(
		imagePath,
		"oras",
		"push",
		fmt.Sprintf("%s:%s", fmt.Sprintf("%s/%s", strings.TrimPrefix(fixture.OCIHostURL, "oci://"), filepath.Base(pathName)), tag),
		".",
	))
}

// PushImageToAuthenticatedOCIRegistry adds a helm chart to helm OCI registry
func PushImageToAuthenticatedOCIRegistry(t *testing.T, pathName, tag string) {
	t.Helper()
	imagePath := "./" + pathName

	errors.NewHandler(t).FailOnErr(fixture.Run(
		imagePath,
		"oras",
		"push",
		fmt.Sprintf("%s:%s", fmt.Sprintf("%s/%s", strings.TrimPrefix(fixture.AuthenticatedOCIHostURL, "oci://"), filepath.Base(pathName)), tag),
		".",
	))
}

// AddWriteCredentials adds write credentials for a repository.
// Write credentials are used by the commit-server to push hydrated manifests back to the repository.
// TODO: add CLI support for managing write credentials and use that here instead.
func AddWriteCredentials(t *testing.T, name string, insecure bool, repoURLType fixture.RepoURLType) {
	t.Helper()
	repoURL := fixture.RepoURL(repoURLType)

	// Create a Kubernetes secret with the repository-write label
	// Replace invalid characters for secret name
	secretName := "write-creds-" + name

	_, err := fixture.KubeClientset.CoreV1().Secrets(fixture.ArgoCDNamespace).Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
				Labels: map[string]string{
					common.LabelKeySecretType: common.LabelValueSecretTypeRepositoryWrite,
				},
			},
			StringData: map[string]string{
				"url":      repoURL,
				"username": fixture.GitUsername,
				"password": fixture.GitPassword,
				"insecure": strconv.FormatBool(insecure),
			},
		},
		metav1.CreateOptions{},
	)
	require.NoError(t, err)
}
