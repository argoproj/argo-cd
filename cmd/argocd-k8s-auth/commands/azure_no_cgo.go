//go:build darwin && !cgo

// Package commands
// This file is used when the GOOS is darwin and CGO is not enabled.
// It provides a no-op implementation of newAzureCommand to allow goreleaser to build
// a darwin binary on a linux machine.
package commands

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
)

func newAzureCommand() *cobra.Command {
	command := &cobra.Command{
		Use: "azure",
		Run: func(c *cobra.Command, _ []string) {
			log.Fatalf(workloadidentity.CGOError)
		},
	}
	return command
}
