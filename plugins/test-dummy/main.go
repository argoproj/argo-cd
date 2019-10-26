package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	cmd := cobra.Command{
		Use: "test-dummy",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use: "schema",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(`{}`)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "template",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(`kind: ConfigMap
apiVersion: v1
metadata:
  name: "%s"
  annotations:
    GitAskpass: "%s"
    GitUsername: "%s"
    GitPassword: "%s"
`,
				os.Getenv("ARGOCD_APP_NAME"),
				os.Getenv("GIT_ASKPASS"),
				os.Getenv("GIT_USERNAME"),
				os.Getenv("GIT_PASSWORD"),
			)
		},
	})
	err := cmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
