package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/util/localconfig"
)

// NewContextCommand returns a new instance of an `argocd ctx` command
func NewContextCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx"},
		Short:   "Switch between contexts",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				printArgoCDContexts(clientOpts.ConfigPath)
				return
			}
			ctxName := args[0]
			argoCDDir, err := localconfig.DefaultConfigDir()
			errors.CheckError(err)
			prevCtxFile := path.Join(argoCDDir, ".prev-ctx")

			if ctxName == "-" {
				prevCtxBytes, err := ioutil.ReadFile(prevCtxFile)
				errors.CheckError(err)
				ctxName = string(prevCtxBytes)
			}
			localCfg, err := localconfig.ReadLocalConfig(clientOpts.ConfigPath)
			errors.CheckError(err)
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
			err = ioutil.WriteFile(prevCtxFile, []byte(prevCtx), 0644)
			errors.CheckError(err)
			fmt.Printf("Switched to context '%s'\n", localCfg.CurrentContext)
		},
	}
	return command
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
