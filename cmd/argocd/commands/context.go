package commands

import (
	stderrors "errors"
	"fmt"
	"os"
	"path"
	"strings"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

// NewContextCommand returns a new instance of an `argocd ctx` command
func NewContextCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:     "context [CONTEXT]",
		Aliases: []string{"ctx"},
		Short:   "Switch between contexts",
		Example: `# List Argo CD Contexts
argocd context list

# Switch Argo CD context
argocd context switch cd.argoproj.io

# Delete Argo CD context
argocd context delete cd.argoproj.io`,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewContextListCommand(clientOpts))
	command.AddCommand(NewContextSwitchCommand(clientOpts))
	command.AddCommand(NewContextDeleteCommand(clientOpts))
	command.AddCommand(NewContextLoginCommand(clientOpts))
	return command
}

func NewContextListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "list",
		Short: "List ArgoCD Contexts",
		Example: `   # List ArgoCD Contexts
	argocd context list`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			printArgoCDContexts(clientOpts.ConfigPath)
		},
	}

	return command
}

func NewContextSwitchCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "switch",
		Short: "Switch ArgoCD Context",
		Example: `   # Switch ArgoCD Context
	argocd context switch cd.argoproj.io`,
		Run: func(c *cobra.Command, args []string) {
			localCfg, err := localconfig.ReadLocalConfig(clientOpts.ConfigPath)
			errors.CheckError(err)

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			ctxName := args[0]

			argoCDDir, err := localconfig.DefaultConfigDir()
			errors.CheckError(err)
			prevCtxFile := path.Join(argoCDDir, ".prev-ctx")

			if ctxName == "-" {
				prevCtxBytes, err := os.ReadFile(prevCtxFile)
				errors.CheckError(err)
				ctxName = string(prevCtxBytes)
			}
			if localCfg.CurrentContext == ctxName {
				fmt.Printf("Already at context '%s'\n", localCfg.CurrentContext)
				return
			}
			if _, err = localCfg.ResolveContext(ctxName); err != nil {
				log.Fatal(err)
			}
			prevCtx := localCfg.CurrentContext
			localCfg.CurrentContext = ctxName

			err = localconfig.WriteLocalConfig(*localCfg, clientOpts.ConfigPath)
			errors.CheckError(err)
			err = os.WriteFile(prevCtxFile, []byte(prevCtx), 0o644)
			errors.CheckError(err)
			fmt.Printf("Switched to context '%s'\n", localCfg.CurrentContext)
		},
	}
	return command
}

func NewContextLoginCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "login",
		Short: "Login using ArgoCD Context",
		Example: `  # Login using ArgoCD Context
	argocd context login cd.argoproj.io`,
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			localCfg, err := localconfig.ReadLocalConfig(clientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				return fmt.Errorf("couldn't find local config")
			}
			ctx, err := localCfg.GetContext(args[0])
			if err != nil {
				return fmt.Errorf("context %s does not exist", args[0])
			}
			server, err := localCfg.GetServer(ctx.Server)
			if err != nil {
				return fmt.Errorf("server %s does not exist", ctx.Server)
			}
			clientOpts.ServerAddr = server.Server
			clientOpts.Context = ctx.Name
			loginCmd := NewLoginCommand(clientOpts)
			loginCmd.SetArgs([]string{server.Server})
			err = loginCmd.Execute()
			errors.CheckError(err)
			return nil
		},
	}
	return command
}

func NewContextDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "delete",
		Short: "Delete ArgoCD Context",
		Example: `  # Delete ArgoCD Context
	argocd context delete cd.argoproj.io`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			err := deleteContext(args[0], clientOpts.ConfigPath)
			errors.CheckError(err)
		},
	}
	return command
}

func deleteContext(context, configPath string) error {
	localCfg, err := localconfig.ReadLocalConfig(configPath)
	errors.CheckError(err)
	if localCfg == nil {
		return stderrors.New("nothing to logout from")
	}
	serverName, ok := localCfg.RemoveContext(context)
	if !ok {
		return fmt.Errorf("context %s does not exist", context)
	}
	_ = localCfg.RemoveUser(context)
	_ = localCfg.RemoveServer(serverName)

	if localCfg.IsEmpty() {
		err = localconfig.DeleteLocalConfig(configPath)
		errors.CheckError(err)
	} else {
		if localCfg.CurrentContext == context {
			localCfg.CurrentContext = ""
		}
		err = localconfig.ValidateLocalConfig(*localCfg)
		if err != nil {
			return stderrors.New("error in logging out")
		}
		err = localconfig.WriteLocalConfig(*localCfg, configPath)
		errors.CheckError(err)
	}
	fmt.Printf("Context '%s' deleted\n", context)
	return nil
}

func printArgoCDContexts(configPath string) {
	localCfg, err := localconfig.ReadLocalConfig(configPath)
	errors.CheckError(err)
	if localCfg == nil {
		log.Fatalf("No contexts defined in %s", configPath)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()
	columnNames := []string{"CURRENT", "NAME", "SERVER"}
	_, err = fmt.Fprintf(w, "%s\n", strings.Join(columnNames, "\t"))
	errors.CheckError(err)

	for _, contextRef := range localCfg.Contexts {
		context, err := localCfg.ResolveContext(contextRef.Name)
		if err != nil {
			log.Warnf("Context '%s' had error: %v", contextRef.Name, err)
		}
		prefix := " "
		if localCfg.CurrentContext == context.Name {
			prefix = "*"
		}
		_, err = fmt.Fprintf(w, "%s\t%s\t%s\n", prefix, context.Name, context.Server.Server)
		errors.CheckError(err)
	}
}
