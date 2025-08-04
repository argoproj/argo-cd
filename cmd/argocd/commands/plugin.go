package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v3/util/cli"

	log "github.com/sirupsen/logrus"
)

// DefaultPluginHandler implements the PluginHandler interface
type DefaultPluginHandler struct {
	ValidPrefixes []string
	lookPath      func(file string) (string, error)
	run           func(cmd *exec.Cmd) error
}

// NewDefaultPluginHandler instantiates the DefaultPluginHandler
func NewDefaultPluginHandler(validPrefixes []string) *DefaultPluginHandler {
	return &DefaultPluginHandler{
		ValidPrefixes: validPrefixes,
		lookPath:      exec.LookPath,
		run: func(cmd *exec.Cmd) error {
			return cmd.Run()
		},
	}
}

// HandleCommandExecutionError processes the error returned from executing the command.
// It handles both standard Argo CD commands and plugin commands. We don't require to return
// error but we are doing it to cover various test scenarios.
func (h *DefaultPluginHandler) HandleCommandExecutionError(err error, isArgocdCLI bool, args []string) error {
	// the log level needs to be setup manually here since the initConfig()
	// set by the cobra.OnInitialize() was never executed because cmd.Execute()
	// gave us a non-nil error.
	initConfig()
	cli.SetLogFormat("text")
	// If it's an unknown command error, attempt to handle it as a plugin.
	// Unfortunately, cobra doesn't handle this error, so we need to assume
	// that error consists of substring "unknown command".
	// https://github.com/spf13/cobra/pull/2167
	if isArgocdCLI && strings.Contains(err.Error(), "unknown command") {
		pluginPath, pluginErr := h.handlePluginCommand(args[1:])
		// IMP: If a plugin doesn't exist, the returned path will be empty along with nil error
		// This means the command is neither a normal Argo CD Command nor a plugin.
		if pluginErr != nil {
			// If plugin handling fails, report the plugin error and exit
			fmt.Printf("Error: %v\n", pluginErr)
			return pluginErr
		} else if pluginPath == "" {
			fmt.Printf("Error: %v\nRun 'argocd --help' for usage.\n", err)
			return err
		}
	} else {
		// If it's any other error (not an unknown command), report it directly and exit
		fmt.Printf("Error: %v\n", err)
		return err
	}

	return nil
}

// handlePluginCommand is  responsible for finding and executing a plugin when a command isn't recognized as a built-in command
func (h *DefaultPluginHandler) handlePluginCommand(cmdArgs []string) (string, error) {
	foundPluginPath := ""
	path, found := h.lookForPlugin(cmdArgs[0])
	if !found {
		return foundPluginPath, nil
	}

	foundPluginPath = path

	// Execute the plugin that is found
	if err := h.executePlugin(foundPluginPath, cmdArgs[1:], os.Environ()); err != nil {
		return foundPluginPath, err
	}

	return foundPluginPath, nil
}

// lookForPlugin looks for a plugin in the PATH that starts with argocd prefix
func (h *DefaultPluginHandler) lookForPlugin(filename string) (string, bool) {
	for _, prefix := range h.ValidPrefixes {
		pluginName := fmt.Sprintf("%s-%s", prefix, filename)
		path, err := h.lookPath(pluginName)
		if err != nil {
			//  error if a plugin is found in a relative path
			if errors.Is(err, exec.ErrDot) {
				log.Errorf("Plugin '%s' found in relative path: %v", pluginName, err)
			} else {
				log.Warnf("error looking for plugin '%s': %v", pluginName, err)
			}
			continue
		}

		if path == "" {
			return "", false
		}

		return path, true
	}

	return "", false
}

// executePlugin implements PluginHandler and executes a plugin found
func (h *DefaultPluginHandler) executePlugin(executablePath string, cmdArgs, environment []string) error {
	cmd := h.command(executablePath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = environment

	err := h.run(cmd)
	if err != nil {
		return err
	}

	return nil
}

// command creates a new command for all OSs
func (h *DefaultPluginHandler) command(name string, arg ...string) *exec.Cmd {
	cmd := &exec.Cmd{
		Path: name,
		Args: append([]string{name}, arg...),
	}
	if filepath.Base(name) == name {
		lp, err := h.lookPath(name)
		if lp != "" && err != nil {
			// Update cmd.Path even if err is non-nil.
			// If err is ErrDot (especially on Windows), lp may include a resolved
			// extension (like .exe or .bat) that should be preserved.
			cmd.Path = lp
		}
	}
	return cmd
}
