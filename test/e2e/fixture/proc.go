package fixture

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Constants for the names of processes common in both local and container Procfiles
// Also includes some constants for the binary name and the file to hold environment variables
const (
	ApplicationControllerProcName    = "controller"
	APIServerProcName                = "api-server"
	DexProcName                      = "dex"
	RedisProcName                    = "redis"
	RepoServerProcName               = "repo-server"
	CommitServerProcName             = "commit-server"
	UIProcName                       = "ui"
	HelmRegistryProcName             = "helm-registry"
	DevMounterProcName               = "dev-mounter"
	ApplicationSetControllerProcName = "applicationset-controller"
	NotificationServerProcName       = "notification"
	procfileBinary                   = "goreman"
	e2eEnvVariableFilePath           = "/tmp/argocd-e2e-env"
)

// StartProcess allows you to start a procress that is not running
func StartProcess(processName string) error {
	cmd := exec.CommandContext(context.Background(), procfileBinary, "run", "start", processName)
	return cmd.Run()
}

// StartProcessWithEnv updates the .env file that is created for the e2e tests with the provided
// values and then restarts the component.
// At the moment this only supports the application controller
func StartProcessWithEnv(processName string, env map[string]string) error {
	var envVariables strings.Builder
	for k, v := range env {
		fmt.Fprintf(&envVariables, "export %s=%s\n", k, v)
	}

	err := os.WriteFile(e2eEnvVariableFilePath, []byte(envVariables.String()), 0o644)
	if err != nil {
		return err
	}

	return StartProcess(processName)
}

// StopProcess stops the specified process and resets it's environment variables
func StopProcess(processName string) error {
	// Delete env file if it exists in tmp dir
	if _, err := os.Stat(e2eEnvVariableFilePath); err == nil {
		err := os.Remove(e2eEnvVariableFilePath)
		if err != nil {
			return err
		}
	}

	cmd := exec.CommandContext(context.Background(), procfileBinary, "run", "stop", processName)
	return cmd.Run()
}

// IsProcessRunning checks to see if the process is running
func IsProcessRunning(processName string) (bool, error) {
	cmd := exec.CommandContext(context.Background(), procfileBinary, "run", "status")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	sc := bufio.NewScanner(bytes.NewReader(output))
	for sc.Scan() {
		t := sc.Text()
		if len(t) > 1 && t[1:] == processName {
			return t[0] == '*', nil
		}
	}
	return false, nil
}

// RestartProcess restarts a procress with or without new environment variables
func RestartProcess(processName string, env map[string]string) error {
	if running, err := IsProcessRunning(processName); !running {
		if err != nil {
			return err
		}
		return StartProcessWithEnv(processName, env)
	}

	err := StopProcess(processName)
	if err != nil {
		return err
	}

	if env != nil {
		return StartProcessWithEnv(processName, env)
	}
	return StartProcess(processName)
}
