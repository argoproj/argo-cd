package fixture

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"
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
func StartProcess(process string) error {
	log.Infof("starting process: %s", process)
	cmd := exec.CommandContext(context.Background(), procfileBinary, "run", "start", process)
	return cmd.Run()
}

// StartProcessWithEnv updates the .env file that is created for the e2e tests with the provided
// values and then restarts the component.
// At the moment this only supports the application controller
func StartProcessWithEnv(process string, env map[string]string) error {
	var envVariables strings.Builder
	fields := make(log.Fields, len(env))
	for k, v := range env {
		fmt.Fprintf(&envVariables, "export %s=%s\n", k, v)
		fields[k] = v
	}

	log.WithFields(fields).Infof("starting process with env: %s", process)

	err := os.WriteFile(e2eEnvVariableFilePath, []byte(envVariables.String()), 0o644)
	if err != nil {
		return err
	}

	return StartProcess(process)
}

// StopProcess stops the specified process and resets it's environment variables
func StopProcess(process string) error {
	log.Infof("stopping process: %s", process)
	// Delete env file if it exists in tmp dir
	if _, err := os.Stat(e2eEnvVariableFilePath); err == nil {
		err := os.Remove(e2eEnvVariableFilePath)
		if err != nil {
			return err
		}
	}

	cmd := exec.CommandContext(context.Background(), procfileBinary, "run", "stop", process)
	return cmd.Run()
}

// IsProcessRunning checks to see if the process is running
func IsProcessRunning(process string) (bool, error) {
	cmd := exec.CommandContext(context.Background(), procfileBinary, "run", "status")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	sc := bufio.NewScanner(bytes.NewReader(output))
	for sc.Scan() {
		t := sc.Text()
		if len(t) > 1 && t[1:] == process {
			return t[0] == '*', nil
		}
	}
	return false, nil
}

// RestartProcess restarts a procress with or without new environment variables
func RestartProcess(process string, env map[string]string) error {
	startFunc := func() error {
		return StartProcess(process)
	}
	if env != nil {
		startFunc = func() error {
			return StartProcessWithEnv(process, env)
		}
	}

	if running, err := IsProcessRunning(process); !running {
		if err != nil {
			return err
		}
		return startFunc()
	}

	err := StopProcess(process)
	if err != nil {
		return err
	}

	return startFunc()
}

// EnsureProcessesAreRunning checks to see if processes are running and then starts them if they are not
func EnsureProcessesAreRunning(procs []string) error {
	cmd := exec.CommandContext(context.Background(), procfileBinary, "run", "status")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(bytes.NewReader(output))
	for sc.Scan() {
		t := sc.Text()

		if len(t) > 1 && t[0] == ' ' && slices.Contains(procs, t[1:]) {
			err = StartProcess(t[1:])
			if err != nil {
				return err
			}
		}
	}
	return nil
}
