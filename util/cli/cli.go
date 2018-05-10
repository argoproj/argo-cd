// Package cmd provides functionally common to various argo CLIs

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	argocd "github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/client-go/tools/clientcmd"
)

// NewVersionCmd returns a new `version` command to be used as a sub-command to root
func NewVersionCmd(cliName string) *cobra.Command {
	var short bool
	versionCmd := cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Print version information"),
		Run: func(cmd *cobra.Command, args []string) {
			version := argocd.GetVersion()
			fmt.Printf("%s: %s\n", cliName, version)
			if short {
				return
			}
			fmt.Printf("  BuildDate: %s\n", version.BuildDate)
			fmt.Printf("  GitCommit: %s\n", version.GitCommit)
			fmt.Printf("  GitTreeState: %s\n", version.GitTreeState)
			if version.GitTag != "" {
				fmt.Printf("  GitTag: %s\n", version.GitTag)
			}
			fmt.Printf("  GoVersion: %s\n", version.GoVersion)
			fmt.Printf("  Compiler: %s\n", version.Compiler)
			fmt.Printf("  Platform: %s\n", version.Platform)
		},
	}
	versionCmd.Flags().BoolVar(&short, "short", false, "print just the version number")
	return &versionCmd
}

// AddKubectlFlagsToCmd adds kubectl like flags to a command and returns the ClientConfig interface
// for retrieving the values.
func AddKubectlFlagsToCmd(cmd *cobra.Command) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	kflags := clientcmd.RecommendedConfigOverrideFlags("")
	cmd.PersistentFlags().StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to a kube config. Only required if out-of-cluster")
	clientcmd.BindOverrideFlags(&overrides, cmd.PersistentFlags(), kflags)
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
}

// PromptCredentials is a helper to prompt the user for a username and password (unless already supplied)
func PromptCredentials(username, password string) (string, string) {
	return PromptUsername(username), PromptPassword(password)
}

// PromptUsername prompts the user for a username value
func PromptUsername(username string) string {
	return PromptMessage("Username", username)
}

// PromptMessage prompts the user for a value (unless already supplied)
func PromptMessage(message, value string) string {
	for value == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(message + ": ")
		valueRaw, err := reader.ReadString('\n')
		errors.CheckError(err)
		value = strings.TrimSpace(valueRaw)
	}
	return value
}

// PromptPassword prompts the user for a password, without local echo. (unless already supplied)
func PromptPassword(password string) string {
	for password == "" {
		fmt.Print("Password: ")
		passwordRaw, err := terminal.ReadPassword(syscall.Stdin)
		errors.CheckError(err)
		password = string(passwordRaw)
		fmt.Print("\n")
	}
	return password
}

// AskToProceed prompts the user with a message (typically a yes or no question) and returns whether
// or not they responded in the affirmative or negative.
func AskToProceed(message string) bool {
	for {
		fmt.Print(message)
		reader := bufio.NewReader(os.Stdin)
		proceedRaw, err := reader.ReadString('\n')
		errors.CheckError(err)
		switch strings.ToLower(strings.TrimSpace(proceedRaw)) {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}
