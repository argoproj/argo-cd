package commands

import (
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
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

__argocd_list_app_history() {
	local app=$1
	local -a argocd_out
	if argocd_out=($(argocd app history $app --output id 2>/dev/null)); then
		COMPREPLY+=( $( compgen -W "${argocd_out[*]}" -- "$cur" ) )
	fi
}

__argocd_app_rollback() {
	local -a command
	for comp_word in "${COMP_WORDS[@]}"; do
		if [[ $comp_word =~ ^-.*$ ]]; then
			continue
		fi
		command+=($comp_word)
	done

	# fourth arg is app (if present): e.g.- argocd app rollback guestbook
	local app=${command[3]}
	local id=${command[4]}
	if [[ -z $app || $app == $cur ]]; then
		__argocd_list_apps
	elif [[ -z $id || $id == $cur ]]; then
		__argocd_list_app_history $app
	fi
}

__argocd_list_servers() {
	local -a argocd_out
	if argocd_out=($(argocd cluster list --output server 2>/dev/null)); then
		COMPREPLY+=( $( compgen -W "${argocd_out[*]}" -- "$cur" ) )
	fi
}

__argocd_list_repos() {
	local -a argocd_out
	if argocd_out=($(argocd repo list --output url 2>/dev/null)); then
		COMPREPLY+=( $( compgen -W "${argocd_out[*]}" -- "$cur" ) )
	fi
}

__argocd_list_projects() {
	local -a argocd_out
	if argocd_out=($(argocd proj list --output name 2>/dev/null)); then
		COMPREPLY+=( $( compgen -W "${argocd_out[*]}" -- "$cur" ) )
	fi
}

__argocd_list_namespaces() {
	local -a argocd_out
	if argocd_out=($(kubectl get namespaces --no-headers 2>/dev/null | cut -f1 -d' ' 2>/dev/null)); then
		COMPREPLY+=( $( compgen -W "${argocd_out[*]}" -- "$cur" ) )
	fi
}

__argocd_proj_server_namespace() {
	local -a command
	for comp_word in "${COMP_WORDS[@]}"; do
		if [[ $comp_word =~ ^-.*$ ]]; then
			continue
		fi
		command+=($comp_word)
	done

	# expect something like this: argocd proj add-destination PROJECT SERVER NAMESPACE
	local project=${command[3]}
	local server=${command[4]}
	local namespace=${command[5]}
	if [[ -z $project || $project == $cur ]]; then
		__argocd_list_projects
	elif [[ -z $server || $server == $cur ]]; then
		__argocd_list_servers
	elif [[ -z $namespace || $namespace == $cur ]]; then
		__argocd_list_namespaces
	fi
}

__argocd_list_project_role() {
	local project="$1"
	local -a argocd_out
	if argocd_out=($(argocd proj role list "$project" --output=name 2>/dev/null)); then
		COMPREPLY+=( $( compgen -W "${argocd_out[*]}" -- "$cur" ) )
	fi
}

__argocd_proj_role(){
	local -a command
	for comp_word in "${COMP_WORDS[@]}"; do
		if [[ $comp_word =~ ^-.*$ ]]; then
			continue
		fi
		command+=($comp_word)
	done

	# expect something like this: argocd proj role add-policy PROJECT ROLE-NAME
	local project=${command[4]}
	local role=${command[5]}
	if [[ -z $project || $project == $cur ]]; then
		__argocd_list_projects
	elif [[ -z $role || $role == $cur ]]; then
		__argocd_list_project_role $project
	fi
}

__argocd_custom_func() {
	case ${last_command} in
		argocd_app_delete | \
		argocd_app_diff | \
		argocd_app_edit | \
		argocd_app_get | \
		argocd_app_history | \
		argocd_app_manifests | \
		argocd_app_patch-resource | \
		argocd_app_set | \
		argocd_app_sync | \
		argocd_app_terminate-op | \
		argocd_app_unset | \
		argocd_app_wait | \
		argocd_app_create)
			__argocd_list_apps
			return
			;;
		argocd_app_rollback)
			__argocd_app_rollback
			return
			;;
		argocd_cluster_get | \
		argocd_cluster_rm | \
		argocd_cluster_set | \
		argocd_login | \
		argocd_cluster_add)
			__argocd_list_servers
			return
			;;
		argocd_repo_rm | \
		argocd_repo_add)
			__argocd_list_repos
			return
			;;
		argocd_proj_add-destination | \
		argocd_proj_remove-destination)
			__argocd_proj_server_namespace
			return
			;;
		argocd_proj_add-source | \
		argocd_proj_remove-source | \
		argocd_proj_allow-cluster-resource | \
		argocd_proj_allow-namespace-resource | \
		argocd_proj_deny-cluster-resource | \
		argocd_proj_deny-namespace-resource | \
		argocd_proj_delete | \
		argocd_proj_edit | \
		argocd_proj_get | \
		argocd_proj_set | \
		argocd_proj_role_list)
			__argocd_list_projects
			return
			;;
		argocd_proj_role_remove-policy | \
		argocd_proj_role_add-policy | \
		argocd_proj_role_create | \
		argocd_proj_role_delete | \
		argocd_proj_role_get | \
		argocd_proj_role_create-token | \
		argocd_proj_role_delete-token)
			__argocd_proj_role
			return
			;;
		*)
			;;
	esac
}
	`
)

func NewCompletionCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "completion SHELL",
		Short: "output shell completion code for the specified shell (bash, zsh or fish)",
		Long: `Write bash, zsh or fish shell completion code to standard output.

For bash, ensure you have bash completions installed and enabled.
To access completions in your current shell, run
$ source <(argocd completion bash)
Alternatively, write it to a file and source in .bash_profile

For zsh, add the following to your ~/.zshrc file:
source <(argocd completion zsh)
compdef _argocd argocd

Optionally, also add the following, in case you are getting errors involving compdef & compinit such as command not found: compdef:
autoload -Uz compinit
compinit 
`,
		Example: `# For bash
$ source <(argocd completion bash)

# For zsh
$ argocd completion zsh > _argocd
$ source _argocd

# For fish
$ argocd completion fish > ~/.config/fish/completions/argocd.fish
$ source ~/.config/fish/completions/argocd.fish

`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.HelpFunc()(cmd, args)
				os.Exit(1)
			}
			shell := args[0]
			rootCommand := NewCommand()
			rootCommand.BashCompletionFunction = bashCompletionFunc
			availableCompletions := map[string]func(out io.Writer, cmd *cobra.Command) error{
				"bash": runCompletionBash,
				"zsh":  runCompletionZsh,
				"fish": runCompletionFish,
			}
			completion, ok := availableCompletions[shell]
			if !ok {
				fmt.Printf("Invalid shell '%s'. The supported shells are bash, zsh and fish.\n", shell)
				os.Exit(1)
			}
			if err := completion(os.Stdout, rootCommand); err != nil {
				log.Fatal(err)
			}
		},
	}

	return command
}

func runCompletionBash(out io.Writer, cmd *cobra.Command) error {
	return cmd.GenBashCompletion(out)
}

func runCompletionZsh(out io.Writer, cmd *cobra.Command) error {
	return cmd.GenZshCompletion(out)
}

func runCompletionFish(out io.Writer, cmd *cobra.Command) error {
	return cmd.GenFishCompletion(out, true)
}
