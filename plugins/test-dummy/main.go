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
		Use: "get spec",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(``)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "template",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(`kind: ConfigMap
apiVersion: v1
metadata:
  name: $ARGOCD_APP_NAME
  namespace: $ARGOCD_APP_NAMESPACE
  annotations:
    GitAskpass: $GIT_ASKPASS
    GitUsername: $GIT_USERNAME
    GitPassword: $GIT_PASSWORD
`)
		},
	})

	err := cmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
