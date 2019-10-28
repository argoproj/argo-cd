package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/kubernetes/pkg/util/slice"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util/helm"
)

type HelmParameter struct {
	ForceString bool
	Name        string
	Value       string
}

type Spec struct {
	ValueFiles  []string
	Parameters  []HelmParameter
	ReleaseName string
	Values      string
}
type Properties map[string]*Schema

type Schema struct {
	Type       string     `json:"type,omitempty"`
	Title      string     `json:"title,omitempty"`
	Format     string     `json:"format,omitempty"`
	Properties Properties `json:"properties,omitempty"`
	Items      *Schema    `json:"items,omitempty"`
	Enum       []string   `json:"enum,omitempty"`
}

var schema = Schema{
	Type:  "object",
	Title: "HelmAppSpec contains helm app name  in source repo",
	Properties: Properties{
		"name": &Schema{
			Type: "object",
		},
		"parameters": &Schema{
			Type:  "object",
			Title: "HelmParameter is a parameter to a helm template",
			Properties: Properties{
				"forceString": {
					Type:   "boolean",
					Format: "boolean",
					Title:  "ForceString determines whether to tell Helm to interpret booleans and numbers as strings",
				},
				"name": {
					Type:  "string",
					Title: "Name is the name of the helm parameter",
				},
				"value": {
					Type:  "string",
					Title: "Value is the value for the helm parameter",
				},
			},
		},
		"valueFiles": {
			Type: "array",
			Items: &Schema{
				Type: "string",
			},
		},
		"values": {
			Type:  "string",
			Title: "the contents of values.yaml",
		},
	},
}

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
			path := args[0]
			h, err := helm.NewHelmApp(path, nil)
			errors.CheckError(err)
			defer h.Dispose()
			err = h.Init()
			errors.CheckError(err)

			files, err := ioutil.ReadDir(path)
			errors.CheckError(err)
			for _, i := range files {
				n := i.Name()
				if !i.IsDir() && strings.HasPrefix(n, "values") && strings.HasSuffix(n, ".yaml") {
					schema.Properties["valueFiles"].Enum = append(schema.Properties["valueFiles"].Enum, n)
				}
			}
			parameters, err := h.GetParameters([]string{"values.yaml"})
			errors.CheckError(err)

			for name, value := range parameters {
				schema.Properties["parameters"].Enum = append(schema.Properties["parameters"].Enum, name)
				schema.Properties["parameters"].Title = fmt.Sprintf("%s, %s=%s", schema.Properties["parameters"].Title, name, value)
			}

			for name := range schema.Properties {
				slice.SortStrings(schema.Properties[name].Enum)
			}

			output, err := json.Marshal(schema)
			errors.CheckError(err)
			fmt.Println(string(output))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "template",
		Run: func(cmd *cobra.Command, args []string) {
			bytes, err := ioutil.ReadAll(os.Stdin)
			errors.CheckError(err)
			source := &Spec{}
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
