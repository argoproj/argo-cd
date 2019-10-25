package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	cmd := cobra.Command{
		Use: "helm-v3",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use: "get spec",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(`type: object
title: HelmAppSpec contains helm app name  in source repo
properties:
  name:
    type: string
  parameters:
    type: array
    title: the output of 'helm inspect values''
    items:
      type: object
      title: HelmParameter is a parameter to a helm template
      properties:
        forceString:
          type: boolean
          format: boolean
          title: ForceString determines whether to tell Helm to interpret booleans
            and numbers as strings
        name:
          type: string
          title: Name is the name of the helm parameter
        value:
          type: string
          title: Value is the value for the helm parameter
  valueFiles:
    type: array
    items:
      type: string
  values:
    type: string
    title: the contents of values.yaml`)
		},
	})

	err := cmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
