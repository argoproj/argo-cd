package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewClusterCommand returns a new instance of an `argocd cluster` command
func NewClusterCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "cluster",
		Short: fmt.Sprintf("%s cluster (add|list|rm) CLUSTERNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(NewClusterAddCommand())
	command.AddCommand(NewClusterListCommand())
	command.AddCommand(NewClusterRemoveCommand())
	return command
}

// NewClusterAddCommand returns a new instance of an `argocd cluster add` command
func NewClusterAddCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("%s cluster add CLUSTERNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	return command
}

// NewClusterListCommand returns a new instance of an `argocd cluster list` command
func NewClusterListCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm",
		Short: fmt.Sprintf("%s cluster rm CLUSTERNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	return command
}

// NewClusterRemoveCommand returns a new instance of an `argocd cluster rm` command
func NewClusterRemoveCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("%s cluster list", cliName),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	return command
}
