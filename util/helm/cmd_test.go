package helm

import (
	"errors"
	"io"
	"log"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			creds:       &HelmCreds{CAPath: "/path/to/ca"},
			expectedOut: "helm registry login my.registry.com --ca-file /path/to/ca",
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
				CAPath:             "/ca",
				InsecureSkipVerify: true,
			},
			expectedOut:   "helm registry login my.registry.com:5000 --username u --password-stdin --ca-file /ca --insecure",
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
			assert.Equal(t, tc.expectedOut, out)
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
