package helm

import (
	"errors"
	"log"
	"os"
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
	cmd, err := NewCmdWithVersion(".", false, "", "")
	require.NoError(t, err)
	s, _, err := cmd.template("testdata/redis", &TemplateOpts{
		KubeVersion: "1.14",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, s)
}

func TestCmd_template_noApiVersionsInError(t *testing.T) {
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
	_, err := NewCmd(".", "abcd", "", "")
	log.Println(err)
	assert.EqualError(t, err, "helm chart version 'abcd' is not supported")
}

func TestNewCmd_withProxy(t *testing.T) {
	cmd, err := NewCmd(".", "", "https://proxy:8888", ".argoproj.io")
	require.NoError(t, err)
	assert.Equal(t, "https://proxy:8888", cmd.proxy)
	assert.Equal(t, ".argoproj.io", cmd.noProxy)
}

func TestRegistryLogin(t *testing.T) {
	tests := []struct {
		name        string
		repo        string
		creds       *HelmCreds
		execErr     error
		expectedErr error
		expectedOut string
	}{
		{
			name:        "username and password",
			repo:        "my.registry.com/repo",
			creds:       &HelmCreds{Username: "user", Password: "pass"},
			expectedOut: "helm registry login my.registry.com --username user --password pass",
		},
		{
			name:        "username and password with just the hostname",
			repo:        "my.registry.com",
			creds:       &HelmCreds{Username: "user", Password: "pass"},
			expectedOut: "helm registry login my.registry.com --username user --password pass",
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
			name:        "username & password",
			repo:        "my.registry.com/repo",
			creds:       &HelmCreds{Username: "user", Password: "pass"},
			expectedOut: "helm registry login my.registry.com --username user --password pass",
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
			expectedOut: "helm registry login my.registry.com:5000 --username u --password p --ca-file /ca --insecure",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				if tc.execErr != nil {
					return "", tc.execErr
				}
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.RegistryLogin(tc.repo, tc.creds)
			assert.Equal(t, tc.expectedOut, out)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDependencyBuild(t *testing.T) {
	tests := []struct {
		name        string
		insecure    bool
		caFilePath  string
		expectedOut string
	}{
		{
			name:        "without insecure or ca-file",
			insecure:    false,
			caFilePath:  "",
			expectedOut: "helm dependency build",
		},
		{
			name:        "with insecure",
			insecure:    true,
			caFilePath:  "",
			expectedOut: "helm dependency build --insecure-skip-tls-verify",
		},
		{
			name:        "with ca-file",
			insecure:    false,
			caFilePath:  "/path/to/ca.crt",
			expectedOut: "helm dependency build --ca-file /path/to/ca.crt",
		},
		{
			name:        "with insecure and ca-file",
			insecure:    true,
			caFilePath:  "/path/to/ca.crt",
			expectedOut: "helm dependency build --insecure-skip-tls-verify --ca-file /path/to/ca.crt",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newCmdWithVersion(".", false, "", "", func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				return strings.Join(cmd.Args, " "), nil
			})
			require.NoError(t, err)
			out, err := c.dependencyBuild(tc.insecure, tc.caFilePath)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOut, out)
		})
	}
}

func TestWriteCombinedCAFile(t *testing.T) {
	t.Run("empty paths", func(t *testing.T) {
		path, closer, err := writeCombinedCAFile(nil)
		require.NoError(t, err)
		assert.Empty(t, path)
		require.NoError(t, closer.Close())
	})

	t.Run("single CA file", func(t *testing.T) {
		caFile, err := os.CreateTemp(t.TempDir(), "test-ca-*")
		require.NoError(t, err)
		_, err = caFile.WriteString("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n")
		require.NoError(t, err)
		caFile.Close()

		path, closer, err := writeCombinedCAFile([]string{caFile.Name()})
		require.NoError(t, err)
		defer closer.Close()
		assert.NotEmpty(t, path)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(data), "-----BEGIN CERTIFICATE-----")
	})

	t.Run("multiple CA files are concatenated", func(t *testing.T) {
		caFile1, err := os.CreateTemp(t.TempDir(), "test-ca1-*")
		require.NoError(t, err)
		_, err = caFile1.WriteString("-----BEGIN CERTIFICATE-----\nca1\n-----END CERTIFICATE-----\n")
		require.NoError(t, err)
		caFile1.Close()

		caFile2, err := os.CreateTemp(t.TempDir(), "test-ca2-*")
		require.NoError(t, err)
		_, err = caFile2.WriteString("-----BEGIN CERTIFICATE-----\nca2\n-----END CERTIFICATE-----\n")
		require.NoError(t, err)
		caFile2.Close()

		path, closer, err := writeCombinedCAFile([]string{caFile1.Name(), caFile2.Name()})
		require.NoError(t, err)
		defer closer.Close()

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(data), "ca1")
		assert.Contains(t, string(data), "ca2")
	})

	t.Run("duplicate paths are deduplicated", func(t *testing.T) {
		caFile, err := os.CreateTemp(t.TempDir(), "test-ca-*")
		require.NoError(t, err)
		_, err = caFile.WriteString("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n")
		require.NoError(t, err)
		caFile.Close()

		path, closer, err := writeCombinedCAFile([]string{caFile.Name(), caFile.Name()})
		require.NoError(t, err)
		defer closer.Close()

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, 1, strings.Count(string(data), "-----BEGIN CERTIFICATE-----"))
	})

	t.Run("non-existent file is skipped", func(t *testing.T) {
		path, closer, err := writeCombinedCAFile([]string{"/nonexistent/ca.crt"})
		require.NoError(t, err)
		defer closer.Close()
		assert.Empty(t, path)
	})
}

func TestRegistryLogout(t *testing.T) {
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
