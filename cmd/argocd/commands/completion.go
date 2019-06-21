package commands

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
)

const (
	bashCompletionFunc = `
__argocd_list_apps() {
	local -a argocd_out
	if argocd_out=($(argocd app list --output name 2>/dev/null)); then
		COMPREPLY+=( $( compgen -W "${argocd_out[*]}" -- "$cur" ) )
	fi
}

__argocd_app_rollback() {
	# Determine if we already have selected an app
	local -a command
	local app
	for comp_word in "${COMP_WORDS[@]}"; do
		if [[ $comp_word =~ ^-.*$ ]]; then
			continue
		fi
		if [[ $comp_word =~ ^(app|rollback)$ ]]; then
			command+=($comp_word)
		elif [[ "${command[*]}" == "app rollback" ]]; then
			app=$comp_word
			break
		fi
	done

	if [[ -z $app ]]; then
		__argocd_list_apps
	else
		local -a argocd_out
		if argocd_out=($(argocd app history $app --output id 2>/dev/null)); then
			COMPREPLY+=( $( compgen -W "${argocd_out[*]}" -- "$cur" ) )
		fi
	fi
}

__argocd_custom_func() {
	case ${last_command} in
		argocd_app_delete | argocd_app_diff | argocd_app_edit | \
		argocd_app_get | argocd_app_history | argocd_app_manifests | \
		argocd_app_patch-resource | argocd_app_set | argocd_app_sync | \
	    argocd_app_terminate-op | argocd_app_unset | argocd_app_wait)
			__argocd_list_apps
			return
			;;
		argocd_app_rollback)
			__argocd_app_rollback
			return
			;;
		*)
			;;
	esac
}
	`
)

func NewCompletionCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "completion SHELL",
		Short: "output shell completion code for the specified shell (bash or zsh)",
		Long: `Write bash or zsh shell completion code to standard output.

For bash, ensure you have bash completions installed and enabled.
To access completions in your current shell, run
$ source <(argocd completion bash)
Alternatively, write it to a file and source in .bash_profile

For zsh, output to a file in a directory referenced by the $fpath shell
variable.
`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.HelpFunc()(cmd, args)
				os.Exit(1)
			}
			shell := args[0]
			rootCommand := NewCommand()
			rootCommand.BashCompletionFunction = bashCompletionFunc
			availableCompletions := map[string]func(io.Writer) error{
				"bash": rootCommand.GenBashCompletion,
				"zsh":  rootCommand.GenZshCompletion,
			}
			completion, ok := availableCompletions[shell]
			if !ok {
				fmt.Printf("Invalid shell '%s'. The supported shells are bash and zsh.\n", shell)
				os.Exit(1)
			}
			if err := completion(os.Stdout); err != nil {
				log.Fatal(err)
			}
		},
	}

	return command
}
