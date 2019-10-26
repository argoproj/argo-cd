package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/helm"
)

func main() {
	cmd := cobra.Command{
		Use: "helm-v3",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use: "schema",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(`{
  "type": "object",
  "title": "HelmAppSpec contains helm app name  in source repo",
  "properties": {
    "name": {
      "type": "string"
    },
    "parameters": {
      "type": "array",
      "title": "the output of 'helm inspect values''",
      "items": {
        "type": "object",
        "title": "HelmParameter is a parameter to a helm template",
        "properties": {
          "forceString": {
            "type": "boolean",
            "format": "boolean",
            "title": "ForceString determines whether to tell Helm to interpret booleans and numbers as strings"
          },
          "name": {
            "type": "string",
            "title": "Name is the name of the helm parameter"
          },
          "value": {
            "type": "string",
            "title": "Value is the value for the helm parameter"
          }
        }
      }
    },
    "valueFiles": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "values": {
      "type": "string",
      "title": "the contents of values.yaml"
    }
  }
}`)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "template",
		Run: func(cmd *cobra.Command, args []string) {
			bytes, err := ioutil.ReadAll(os.Stdin)
			errors.CheckError(err)
			source := &v1alpha1.ApplicationSourceHelm{}
			err = yaml.Unmarshal(bytes, source)
			errors.CheckError(err)
			app, err := helm.NewHelmApp(".", nil)
			errors.CheckError(err)
			set := map[string]string{}
			setString := map[string]string{}
			for _, p := range source.Parameters {
				if p.ForceString {
					setString[p.Name] = p.Value
				} else {
					set[p.Name] = p.Value
				}
			}
			if source.Values != "" {
				f, err := ioutil.TempFile(".", "helm-v3")
				defer func() { _ = os.RemoveAll(f.Name()) }()
				errors.CheckError(err)
				err = ioutil.WriteFile(f.Name(), []byte(source.Values), 777)
				errors.CheckError(err)
			}
			output, err := app.Template(&helm.TemplateOpts{
				Name:        os.Getenv("ARGOCD_APP_NAME"),
				Namespace:   os.Getenv("ARGOCD_APP_NAMESPACE"),
				KubeVersion: os.Getenv("ARGOCD_KUBE_VERSON"),
				Set:         set,
				SetString:   setString,
				Values:      source.ValueFiles,
			})
			errors.CheckError(err)
			fmt.Println(output)
		},
	})
	err := cmd.Execute()
	errors.CheckError(err)
}
