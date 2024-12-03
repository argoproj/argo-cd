package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type ArgoCDCLIOptions struct {
	PluginHandler PluginHandler
	Arguments     []string
}

// PluginHandler parses command line arguments
// and performs executable filename lookups to search
// for valid plugin files, and execute found plugins.
type PluginHandler interface {
	// LookForPlugin will iterate over a list of given prefixes
	// in order to recognize valid plugin filenames.
	// The first filepath to match a prefix is returned.
	LookForPlugin(filename string) (string, bool)
	// ExecutePlugin receives an executable's filepath, a slice
	// of arguments, and a slice of environment variables
	// to relay to the executable.
	ExecutePlugin(executablePath string, cmdArgs, environment []string) error
}

// DefaultPluginHandler implements the PluginHandler interface
type DefaultPluginHandler struct {
	ValidPrefixes []string
}

// NewDefaultPluginHandler instantiates the DefaultPluginHandler
func NewDefaultPluginHandler(validPrefixes []string) *DefaultPluginHandler {
	return &DefaultPluginHandler{
		ValidPrefixes: validPrefixes,
	}
}

// HandleCommandExecutionError processes the error returned from executing the command.
// It handles both standard Argo CD commands and plugin commands. We don't require to return
// error but we are doing it to cover various test scenarios.
func HandleCommandExecutionError(err error, isArgocdCLI bool, o ArgoCDCLIOptions) error {
	// the log level needs to be setup manually here since the initConfig()
	// set by the cobra.OnInitialize() was never executed because cmd.Execute()
	// gave us a non-nil error.
	initConfig()
	if err != nil {
		// If it's an unknown command error, attempt to handle it as a plugin.
		// Unfortunately, cobra doesn't handle this error, so we need to assume
		// that error consists of substring "unknown command".
		// https://github.com/spf13/cobra/pull/2167
		if isArgocdCLI && strings.Contains(err.Error(), "unknown command") {
			// The PluginPath is important to be returned since it
			// helps us understanding the logic for handling errors.
			log.Println("command does not exist, looking for a plugin...")
			PluginPath, pluginErr := HandlePluginCommand(o.PluginHandler, o.Arguments[1:], 1)
			// IMP: If a plugin doesn't exist, the returned path will be empty along with nil error
			// This means the command is neither a normal Argo CD Command nor a plugin.
			if pluginErr == nil {
				if PluginPath == "" {
					log.Errorf("Error: %v\nRun 'argocd --help' for usage.\n", err)
					return err
				}
			} else {
				// If plugin handling fails, report the plugin error and exit
				log.Errorf("Error: %v\n", pluginErr)
				return pluginErr
			}
		} else {
			// If it's any other error (not an unknown command), report it directly and exit
			log.Errorf("Error: %v\n", err)
			return err
		}
	}

	return nil
}

// HandlePluginCommand is  responsible for finding and executing a plugin when a command isn't recognized as a built-in command
func HandlePluginCommand(pluginHandler PluginHandler, cmdArgs []string, minArgs int) (string, error) {
	var remainingArgs []string // this will contain all "non-flag" arguments
	foundPluginPath := ""
	for _, arg := range cmdArgs {
		// if you encounter a flag, break the loop
		// For eg. If cmdArgs is ["argocd", "foo", "-v"],
		// it will store ["argocd", "foo"] in remainingArgs
		// and stop when it hits the flag -v
		if strings.HasPrefix(arg, "-") {
			break
		}
		remainingArgs = append(remainingArgs, strings.ReplaceAll(arg, "-", "_"))
	}

	if len(remainingArgs) == 0 {
		// the length of cmdArgs is at least 1
		return foundPluginPath, fmt.Errorf("flags cannot be placed before plugin name: %s", cmdArgs[0])
	}

	// try to find the binary, starting at longest possible name with given cmdArgs
	for len(remainingArgs) > 0 {
		path, found := pluginHandler.LookForPlugin(strings.Join(remainingArgs, "-"))
		if !found {
			remainingArgs = remainingArgs[:len(remainingArgs)-1]
			if len(remainingArgs) < minArgs {
				break
			}

			continue
		}

		foundPluginPath = path
		break
	}

	if len(foundPluginPath) == 0 {
		return foundPluginPath, nil
	}

	// Execute the plugin that is found
	if err := pluginHandler.ExecutePlugin(foundPluginPath, cmdArgs[len(remainingArgs):], os.Environ()); err != nil {
		return foundPluginPath, err
	}

	return foundPluginPath, nil
}

// LookForPlugin implements PluginHandler. searches for an executable plugin with a given filename
// and valid prefixes. If the plugin is not found (ErrNotFound), it continues to the next prefix.
// Unexpected errors (e.g., permission issues) are logged for debugging but do not stop the search.
// This doesn't care about the plugin execution errors since those errors are handled separately by
// the execute function.
func (h *DefaultPluginHandler) LookForPlugin(filename string) (string, bool) {
	for _, prefix := range h.ValidPrefixes {
		pluginName := fmt.Sprintf("%s-%s", prefix, filename) // Combine prefix and filename
		path, err := exec.LookPath(pluginName)
		if err != nil || len(path) == 0 {
			continue
		}
		return path, true
	}

	return "", false
}

// ExecutePlugin implements PluginHandler and executes a plugin found
func (h *DefaultPluginHandler) ExecutePlugin(executablePath string, cmdArgs, environment []string) error {
	cmd := Command(executablePath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = environment

	err := cmd.Run()
	if err != nil {
		return err
	}

	// Exit with status 0 if successful, though in most use cases this won't be reached
	return nil
}

// Command creates a new command for all OSs
func Command(name string, arg ...string) *exec.Cmd {
	cmd := &exec.Cmd{
		Path: name,
		Args: append([]string{name}, arg...),
	}
	if filepath.Base(name) == name {
		lp, err := exec.LookPath(name)
		if lp != "" && err != nil {
			// Update cmd.Path even if err is non-nil.
			// If err is ErrDot (especially on Windows), lp may include a resolved
			// extension (like .exe or .bat) that should be preserved.
			cmd.Path = lp
		}
	}
	return cmd
}
