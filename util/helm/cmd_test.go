package helm

import (
	"errors"
	"log"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_cmd_redactor(t *testing.T) {
	require.Equal(t, "--foo bar", redactor("--foo bar"))
	require.Equal(t, "--username ******", redactor("--username bar"))
	require.Equal(t, "--password ******", redactor("--password bar"))
}

func TestCmd_template_kubeVersion(t *testing.T) {
	cmd, err := NewCmdWithVersion(".", false, "", "")
	require.NoError(t, err)
	s, _, err := cmd.template("testdata/redis", &TemplateOpts{
		KubeVersion: "1.14",
	})
	require.NoError(t, err)
	require.NotEmpty(t, s)
}

func TestCmd_template_noApiVersionsInError(t *testing.T) {
	cmd, err := NewCmdWithVersion(".", false, "", "")
	require.NoError(t, err)
	_, _, err = cmd.template("testdata/chart-does-not-exist", &TemplateOpts{
		KubeVersion: "1.14",
		APIVersions: []string{"foo", "bar"},
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "--api-version")
	require.ErrorContains(t, err, "<api versions removed> ")
}

func TestNewCmd_helmInvalidVersion(t *testing.T) {
	_, err := NewCmd(".", "abcd", "", "")
	log.Println(err)
	require.EqualError(t, err, "helm chart version 'abcd' is not supported")
}

func TestNewCmd_withProxy(t *testing.T) {
	cmd, err := NewCmd(".", "", "https://proxy:8888", ".argoproj.io")
	require.NoError(t, err)
	require.Equal(t, "https://proxy:8888", cmd.proxy)
	require.Equal(t, ".argoproj.io", cmd.noProxy)
}

func TestRegistryLogin(t *testing.T) {
	tests := []struct {
		name            string
		repo            string
		creds           *HelmCreds
		expectedErr     error
		runWithRedactor func(*exec.Cmd, func(string) string) (string, error)
	}{
		{
			name:  "username and password",
			repo:  "my.registry.com/repo",
			creds: &HelmCreds{Username: "user", Password: "pass"},
			runWithRedactor: func(cmd *exec.Cmd, _ func(string) string) (string, error) {
				require.Equal(t, []string{"helm", "registry", "login", "my.registry.com", "--username", "user", "--password", "pass"}, cmd.Args)
				return "", nil
			},
		},
		{
			name:  "username and password with just the hostname",
			repo:  "my.registry.com",
			creds: &HelmCreds{Username: "user", Password: "pass"},
			runWithRedactor: func(cmd *exec.Cmd, _ func(string) string) (string, error) {
				require.Equal(t, []string{"helm", "registry", "login", "my.registry.com", "--username", "user", "--password", "pass"}, cmd.Args)
				return "", nil
			},
		},
		{
			name:  "ca file path",
			repo:  "my.registry.com/repo",
			creds: &HelmCreds{CAPath: "/path/to/ca"},
			runWithRedactor: func(cmd *exec.Cmd, _ func(string) string) (string, error) {
				require.Equal(t, []string{"helm", "registry", "login", "my.registry.com", "--ca-file", "/path/to/ca"}, cmd.Args)
				return "", nil
			},
		},
		{
			name:  "insecure skip verify",
			repo:  "my.registry.com/repo",
			creds: &HelmCreds{InsecureSkipVerify: true},
			runWithRedactor: func(cmd *exec.Cmd, _ func(string) string) (string, error) {
				require.Equal(t, []string{"helm", "registry", "login", "my.registry.com", "--insecure"}, cmd.Args)
				return "", nil
			},
		},
		{
			name:  "helm failure",
			repo:  "my.registry.com/repo",
			creds: &HelmCreds{},
			runWithRedactor: func(_ *exec.Cmd, _ func(string) string) (string, error) {
				return "err out", errors.New("exit status 1")
			},
			expectedErr: errors.New("failed to login to registry: failed to get command args to log: exit status 1"),
		},
		{
			name:        "invalid repo",
			repo:        ":///bad-url",
			expectedErr: errors.New("failed to parse OCI repo URL: parse \":///bad-url\": missing protocol scheme"),
		},
		{
			name:  "username & password",
			repo:  "my.registry.com/repo",
			creds: &HelmCreds{Username: "user", Password: "pass"},
			runWithRedactor: func(cmd *exec.Cmd, _ func(string) string) (string, error) {
				require.Equal(t, []string{
					"helm", "registry", "login", "my.registry.com",
					"--username", "user", "--password", "pass",
				}, cmd.Args)
				return "", nil
			},
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
			runWithRedactor: func(cmd *exec.Cmd, _ func(string) string) (string, error) {
				require.Equal(t, []string{
					"helm", "registry", "login", "my.registry.com:5000",
					"--username", "u", "--password", "p",
					"--ca-file", "/ca", "--insecure",
				}, cmd.Args)
				return "", nil
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newCmdWithVersion(".", false, "", "", tc.runWithRedactor)
			require.NoError(t, err)
			_, err = c.RegistryLogin(tc.repo, tc.creds)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRegistryLogout(t *testing.T) {
	tests := []struct {
		name            string
		repo            string
		expectedErr     error
		runWithRedactor func(*exec.Cmd, func(string) string) (string, error)
	}{
		{
			name: "valid repo",
			repo: "my.registry.com/repo",
			runWithRedactor: func(cmd *exec.Cmd, _ func(_ string) string) (string, error) {
				require.Equal(t, []string{"helm", "registry", "logout", "my.registry.com"}, cmd.Args)
				return "", nil
			},
			expectedErr: nil,
		},
		{
			name:        "invalid repo",
			repo:        ":///bad-url",
			expectedErr: errors.New("failed to parse OCI repo URL: parse \":///bad-url\": missing protocol scheme"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newCmdWithVersion(".", false, "", "", tc.runWithRedactor)
			require.NoError(t, err)
			_, err = c.RegistryLogout(tc.repo, nil)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
