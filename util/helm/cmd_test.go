package helm

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

func writeTestCAFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func Test_helmCAFilePathWithSystemTrust_mergesSystemAndCustom(t *testing.T) {
	systemBundle := writeTestCAFile(t, "system.pem", "system-root-ca-bundle\n")
	customCA := writeTestCAFile(t, "custom.pem", "custom-repository-ca\n")
	t.Setenv("SSL_CERT_FILE", systemBundle)

	caFile, closer, err := helmCAFilePathWithSystemTrust(customCA)
	require.NoError(t, err)
	defer utilio.Close(closer)
	require.NotEqual(t, customCA, caFile)

	merged, err := os.ReadFile(caFile)
	require.NoError(t, err)
	assert.Contains(t, string(merged), "system-root-ca-bundle")
	assert.Contains(t, string(merged), "custom-repository-ca")
}

func Test_helmCAFilePathWithSystemTrust_mergesSSLCertDir(t *testing.T) {
	certDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(certDir, "b.pem"), []byte("dir-root-b\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(certDir, "a.pem"), []byte("dir-root-a\n"), 0o644))
	customCA := writeTestCAFile(t, "custom.pem", "custom-repository-ca\n")
	t.Setenv("SSL_CERT_FILE", "")
	t.Setenv("SSL_CERT_DIR", certDir)

	caFile, closer, err := helmCAFilePathWithSystemTrust(customCA)
	require.NoError(t, err)
	defer utilio.Close(closer)

	merged, err := os.ReadFile(caFile)
	require.NoError(t, err)
	assert.Contains(t, string(merged), "dir-root-a")
	assert.Contains(t, string(merged), "dir-root-b")
	assert.Contains(t, string(merged), "custom-repository-ca")
}

func TestFetch_withCAFile_mergesSystemTrust(t *testing.T) {
	systemBundle := writeTestCAFile(t, "system.pem", "system-root-ca-bundle\n")
	repoCA := writeTestCAFile(t, "repo.pem", "repo-ca\n")
	t.Setenv("SSL_CERT_FILE", systemBundle)

	c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
		joined := strings.Join(cmd.Args, " ")
		assert.Contains(t, joined, "--ca-file")
		return joined, nil
	})
	require.NoError(t, err)
	creds := &HelmCreds{CAPath: repoCA}
	out, err := c.Fetch("https://charts.example.com", "mychart", "1.0.0", "/tmp/dest", creds, false)
	require.NoError(t, err)
	assert.Contains(t, out, "--ca-file")
}

func TestRepoAdd_persistsMergedCAFile(t *testing.T) {
	systemBundle := writeTestCAFile(t, "system.pem", "system-root-ca-bundle\n")
	repoCA := writeTestCAFile(t, "repo.pem", "repo-ca\n")
	t.Setenv("SSL_CERT_FILE", systemBundle)

	var caFileFromHelm string
	c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
		for i, arg := range cmd.Args {
			if arg == "--ca-file" && i+1 < len(cmd.Args) {
				caFileFromHelm = cmd.Args[i+1]
			}
		}
		return "added", nil
	})
	require.NoError(t, err)
	creds := &HelmCreds{CAPath: repoCA}
	_, err = c.RepoAdd("testrepo", "https://charts.example.com", creds, false)
	require.NoError(t, err)
	require.NotEmpty(t, caFileFromHelm)
	assert.FileExists(t, caFileFromHelm)
	merged, err := os.ReadFile(caFileFromHelm)
	require.NoError(t, err)
	assert.Contains(t, string(merged), "system-root-ca-bundle")
	assert.Contains(t, string(merged), "repo-ca")
}

func TestPullOCI_withCAFile_mergesSystemTrust(t *testing.T) {
	systemBundle := writeTestCAFile(t, "system.pem", "system-root-ca-bundle\n")
	repoCA := writeTestCAFile(t, "repo.pem", "repo-ca\n")
	t.Setenv("SSL_CERT_FILE", systemBundle)

	c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
		joined := strings.Join(cmd.Args, " ")
		assert.Contains(t, joined, "--ca-file")
		return joined, nil
	})
	require.NoError(t, err)
	creds := &HelmCreds{CAPath: repoCA}
	out, err := c.PullOCI("my.registry.com/myrepo", "mychart", "1.0.0", "/tmp/dest", creds, false)
	require.NoError(t, err)
	assert.Contains(t, out, "--ca-file")
}

func Test_cmd_redactor(t *testing.T) {
	assert.Equal(t, "--foo bar", redactor("--foo bar"))
	assert.Equal(t, "--username ******", redactor("--username bar"))
	assert.Equal(t, "--password ******", redactor("--password bar"))
}

func TestCmd_template_kubeVersion(t *testing.T) {
	t.Parallel()
	cmd, err := NewCmdWithVersion(".", false, "", "")
	require.NoError(t, err)
	s, _, err := cmd.template("testdata/redis", &TemplateOpts{
		KubeVersion: "1.14",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, s)
}

func TestCmd_template_noApiVersionsInError(t *testing.T) {
	t.Parallel()
	cmd, err := NewCmdWithVersion(".", false, "", "")
	require.NoError(t, err)
	_, _, err = cmd.template("testdata/chart-does-not-exist", &TemplateOpts{
		KubeVersion: "1.14",
		APIVersions: []string{"foo", "bar"},
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "--api-version")
	assert.ErrorContains(t, err, "<api versions removed> ")
}

func TestNewCmd_helmInvalidVersion(t *testing.T) {
	t.Parallel()
	_, err := NewCmd(".", "abcd", "", "")
	log.Println(err)
	assert.EqualError(t, err, "helm version 'abcd' is not supported")
}

func TestNewCmd_withProxy(t *testing.T) {
	t.Parallel()
	cmd, err := NewCmd(".", "", "https://proxy:8888", ".argoproj.io")
	require.NoError(t, err)
	assert.Equal(t, "https://proxy:8888", cmd.proxy)
	assert.Equal(t, ".argoproj.io", cmd.noProxy)
}

func TestRegistryLogin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		repo          string
		creds         *HelmCreds
		plainHTTP     bool
		execErr       error
		expectedErr   error
		expectedOut   string
		expectedStdin string
	}{
		{
			name:          "username and password",
			repo:          "my.registry.com/repo",
			creds:         &HelmCreds{Username: "user", Password: "pass"},
			expectedOut:   "helm registry login my.registry.com --username user --password-stdin",
			expectedStdin: "pass",
		},
		{
			name:          "username and password with just the hostname",
			repo:          "my.registry.com",
			creds:         &HelmCreds{Username: "user", Password: "pass"},
			expectedOut:   "helm registry login my.registry.com --username user --password-stdin",
			expectedStdin: "pass",
		},
		{
			name:        "ca file path",
			repo:        "my.registry.com/repo",
			creds:       func() *HelmCreds { return &HelmCreds{CAPath: writeTestCAFile(t, "ca.pem", "repo-ca\n")} }(),
			expectedOut: "helm registry login my.registry.com --ca-file",
		},
		{
			name:        "insecure skip verify",
			repo:        "my.registry.com/repo",
			creds:       &HelmCreds{InsecureSkipVerify: true},
			expectedOut: "helm registry login my.registry.com --insecure",
		},
		{
			name:        "helm failure",
			repo:        "my.registry.com/repo",
			creds:       &HelmCreds{},
			execErr:     errors.New("exit status 1"),
			expectedErr: errors.New("failed to login to registry: failed running helm: exit status 1"),
		},
		{
			name:        "invalid repo",
			repo:        ":///bad-url",
			expectedErr: errors.New("failed to parse registry URL: parse \":///bad-url\": missing protocol scheme"),
		},
		{
			name:          "username & password",
			repo:          "my.registry.com/repo",
			creds:         &HelmCreds{Username: "user", Password: "pass"},
			expectedOut:   "helm registry login my.registry.com --username user --password-stdin",
			expectedStdin: "pass",
		},
		{
			name: "combined flags",
			repo: "my.registry.com:5000/repo",
			creds: &HelmCreds{
				Username:           "u",
				Password:           "p",
				CAPath:             writeTestCAFile(t, "ca.pem", "repo-ca\n"),
				InsecureSkipVerify: true,
			},
			expectedOut:   "helm registry login my.registry.com:5000 --username u --password-stdin --ca-file",
			expectedStdin: "p",
		},
		{
			name:          "plain-http",
			repo:          "my.registry.com/repo",
			creds:         &HelmCreds{Username: "user", Password: "pass"},
			plainHTTP:     true,
			expectedOut:   "helm registry login my.registry.com --plain-http --username user --password-stdin",
			expectedStdin: "pass",
		},
		{
			name:        "insecure and plain-http both set",
			repo:        "my.registry.com/repo",
			creds:       &HelmCreds{InsecureSkipVerify: true},
			plainHTTP:   true,
			expectedOut: "helm registry login my.registry.com --plain-http --insecure",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				var stdin []byte
				if cmd.Stdin != nil {
					var readErr error
					stdin, readErr = io.ReadAll(cmd.Stdin)
					require.NoError(t, readErr)
				}
				assert.Equal(t, tc.expectedStdin, string(stdin))
				if tc.expectedStdin != "" {
					assert.NotContains(t, cmd.Args, tc.expectedStdin)
				}
				if tc.execErr != nil {
					return "", tc.execErr
				}
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.RegistryLogin(t.Context(), tc.repo, tc.creds, tc.plainHTTP)
			if strings.HasSuffix(tc.expectedOut, "--ca-file") {
				assert.True(t, strings.HasPrefix(out, tc.expectedOut+" "))
				if tc.name == "combined flags" {
					assert.Contains(t, out, "--insecure")
				}
			} else {
				assert.Equal(t, tc.expectedOut, out)
			}
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPullOCI(t *testing.T) {
	tests := []struct {
		name        string
		creds       HelmCreds
		plainHTTP   bool
		expectedOut string
	}{
		{
			name:        "without flags",
			creds:       HelmCreds{},
			plainHTTP:   false,
			expectedOut: "helm pull oci://my.registry.com/myrepo/mychart --version 1.0.0 --destination /tmp/dest",
		},
		{
			name:        "insecure skip verify",
			creds:       HelmCreds{InsecureSkipVerify: true},
			plainHTTP:   false,
			expectedOut: "helm pull oci://my.registry.com/myrepo/mychart --version 1.0.0 --destination /tmp/dest --insecure-skip-tls-verify",
		},
		{
			name:        "plain-http",
			creds:       HelmCreds{},
			plainHTTP:   true,
			expectedOut: "helm pull oci://my.registry.com/myrepo/mychart --version 1.0.0 --destination /tmp/dest --plain-http",
		},
		{
			name:        "insecure and plain-http both set",
			creds:       HelmCreds{InsecureSkipVerify: true},
			plainHTTP:   true,
			expectedOut: "helm pull oci://my.registry.com/myrepo/mychart --version 1.0.0 --destination /tmp/dest --insecure-skip-tls-verify --plain-http",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.PullOCI("my.registry.com/myrepo", "mychart", "1.0.0", "/tmp/dest", &tc.creds, tc.plainHTTP)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOut, out)
		})
	}
}

func TestDependencyBuild(t *testing.T) {
	tests := []struct {
		name        string
		insecure    bool
		plainHTTP   bool
		expectedOut string
	}{
		{
			name:        "without flags",
			insecure:    false,
			plainHTTP:   false,
			expectedOut: "helm dependency build",
		},
		{
			name:        "with insecure",
			insecure:    true,
			plainHTTP:   false,
			expectedOut: "helm dependency build --insecure-skip-tls-verify",
		},
		{
			name:        "with plain-http",
			insecure:    false,
			plainHTTP:   true,
			expectedOut: "helm dependency build --plain-http",
		},
		{
			name:        "with insecure and plain-http both set",
			insecure:    true,
			plainHTTP:   true,
			expectedOut: "helm dependency build --insecure-skip-tls-verify --plain-http",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.dependencyBuild(tc.insecure, tc.plainHTTP)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOut, out)
		})
	}
}

func TestRegistryLogout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		repo        string
		execErr     error
		expectedErr error
		expectedOut string
	}{
		{
			name:        "valid repo",
			repo:        "my.registry.com/repo",
			expectedOut: "helm registry logout my.registry.com",
			expectedErr: nil,
		},
		{
			name:        "invalid repo",
			repo:        ":///bad-url",
			expectedErr: errors.New("failed to parse registry URL: parse \":///bad-url\": missing protocol scheme"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				if tc.execErr != nil {
					return "", tc.execErr
				}
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.RegistryLogout(tc.repo, nil)
			assert.Equal(t, tc.expectedOut, out)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
