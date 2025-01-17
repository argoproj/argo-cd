package util

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBearerTokenAndPasswordCombo(t *testing.T) {
	tests := []struct {
		name        string
		bearerToken string
		password    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Both token and password set",
			bearerToken: "some-token",
			password:    "some-password",
			expectError: true,
			errorMsg:    "only --bearer-token or --password is allowed, not both",
		},
		{
			name:        "Only token set",
			bearerToken: "some-token",
			password:    "",
			expectError: false,
		},
		{
			name:        "Only password set",
			bearerToken: "",
			password:    "some-password",
			expectError: false,
		},
		{
			name:        "Neither token nor password set",
			bearerToken: "",
			password:    "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// if tt.expectError {
			runCmdAndCheckError(t, tt.expectError, "TestValidateBearerTokenAndPasswordCombo", tt.errorMsg, func() {
				ValidateBearerTokenAndPasswordCombo(tt.bearerToken, tt.password)
			})
		})
	}
}

func TestValidateBearerTokenForGitOnly(t *testing.T) {
	tests := []struct {
		name        string
		bearerToken string
		repoType    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Bearer token with helm repo",
			bearerToken: "some-token",
			repoType:    "helm",
			expectError: true,
			errorMsg:    "--bearer-token is only supported for Git repositories",
		},
		{
			name:        "Bearer token with git repo",
			bearerToken: "some-token",
			repoType:    "git",
			expectError: false,
		},
		{
			name:        "No bearer token with helm repo",
			bearerToken: "",
			repoType:    "helm",
			expectError: false,
		},
		{
			name:        "No bearer token with git repo",
			bearerToken: "",
			repoType:    "git",
			expectError: false,
		},
		{
			name:        "Bearer token with empty repo",
			bearerToken: "some-token",
			repoType:    "",
			expectError: true,
			errorMsg:    "--bearer-token is only supported for Git repositories",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCmdAndCheckError(t, tt.expectError, "TestValidateBearerTokenForGitOnly", tt.errorMsg, func() {
				ValidateBearerTokenForGitOnly(tt.bearerToken, tt.repoType)
			})
		})
	}
}

func TestValidateBearerTokenForHTTPSRepoOnly(t *testing.T) {
	tests := []struct {
		name        string
		bearerToken string
		isHTTPS     bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Bearer token with HTTPS repo",
			bearerToken: "some-token",
			isHTTPS:     true,
			expectError: false,
		},
		{
			name:        "Bearer token with non-HTTPS repo",
			bearerToken: "some-token",
			isHTTPS:     false,
			expectError: true,
			errorMsg:    "--bearer-token is only supported for HTTPS repositories",
		},
		{
			name:        "No bearer token with HTTPS repo",
			bearerToken: "",
			isHTTPS:     true,
			expectError: false,
		},
		{
			name:        "No bearer token with non-HTTPS repo",
			bearerToken: "",
			isHTTPS:     false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCmdAndCheckError(t, tt.expectError, "TestValidateBearerTokenForHTTPSRepoOnly", tt.errorMsg, func() {
				ValidateBearerTokenForHTTPSRepoOnly(tt.bearerToken, tt.isHTTPS)
			})
		})
	}
}

func runCmdAndCheckError(t *testing.T, expectError bool, testName, errorMsg string, validationFunc func()) {
	// All CLI commands do not return an error upon failure.
	// Instead, the errors.CheckError(err) in each CLI command performs non-zero code system exit.
	// So in order to test the commands, we need to run the command in a separate process and capture it's error message.
	// https://stackoverflow.com/a/33404435
	// TODO: consider whether to change all the CLI commands to return an error instead of performing a non-zero code system exit.
	if expectError {
		if os.Getenv("BE_CRASHER") == "1" {
			validationFunc()
			return
		}
		cmd := exec.Command(os.Args[0], "-test.run="+testName)
		cmd.Env = append(os.Environ(), "BE_CRASHER=1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && !exitErr.Success() {
			t.Log(stderr.String())
			assert.Contains(t, stderr.String(), errorMsg)
			return
		}
		t.Fatalf("process ran with err %v, want exit status 1", err)
	} else {
		validationFunc()
	}
}
