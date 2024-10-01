package util

import (
	pluginError "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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

// HandlePluginCommand is  responsible for finding and executing a plugin when a command isn't recognized as a built-in command
func HandlePluginCommand(pluginHandler PluginHandler, cmdArgs []string, minArgs int) error {
	var remainingArgs []string // this will contain all "non-flag" arguments
	for _, arg := range cmdArgs {
		// if you encounter a flag, break the loop
		// For eg. If cmdArgs is ["argocd", "foo", "-v"],
		// it will store ["argocd", "foo"] in remainingArgs
		// and stop when it hits the flag -v
		if strings.HasPrefix(arg, "-") {
			break
		}
		remainingArgs = append(remainingArgs, strings.Replace(arg, "-", "_", -1))
	}

	if len(remainingArgs) == 0 {
		// the length of cmdArgs is at least 1
		return fmt.Errorf("flags cannot be placed before plugin name: %s", cmdArgs[0])
	}

	foundPluginPath := ""

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
		return nil
	}

	// Execute the plugin that is found
	if err := pluginHandler.ExecutePlugin(foundPluginPath, cmdArgs[len(remainingArgs):], os.Environ()); err != nil {
		return err
	}

	return nil
}

// LookForPlugin implements PluginHandler
func (h *DefaultPluginHandler) LookForPlugin(filename string) (string, bool) {
	for _, prefix := range h.ValidPrefixes {
		path, err := exec.LookPath(fmt.Sprintf("%s-%s", prefix, filename))
		if shouldSkipOnLookPathErr(err) || len(path) == 0 {
			continue
		}
		return path, true
	}
	return "", false
}

// ExecutePlugin implements PluginHandler and executes a plugin found
func (h *DefaultPluginHandler) ExecutePlugin(executablePath string, cmdArgs, environment []string) error {
	// Windows does not support exec syscall.
	if runtime.GOOS == "windows" {
		cmd := Command(executablePath, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = environment
		err := cmd.Run()
		if err == nil {
			os.Exit(0)
		}
		return err
	}

	return syscall.Exec(executablePath, append([]string{executablePath}, cmdArgs...), environment)
}

// Command creates a new command for windows OS since windows doesn't support exec syscall
func Command(name string, arg ...string) *exec.Cmd {
	cmd := &exec.Cmd{
		Path: name,
		Args: append([]string{name}, arg...),
	}
	if filepath.Base(name) == name {
		lp, err := exec.LookPath(name)
		if lp != "" && !shouldSkipOnLookPathErr(err) {
			// Update cmd.Path even if err is non-nil.
			// If err is ErrDot (especially on Windows), lp may include a resolved
			// extension (like .exe or .bat) that should be preserved.
			cmd.Path = lp
		}
	}
	return cmd
}

// shouldSkipOnLookPathErr checks if the error is nil and it is of type ErrDot
func shouldSkipOnLookPathErr(err error) bool {
	return err != nil && !pluginError.Is(err, exec.ErrDot)
}
