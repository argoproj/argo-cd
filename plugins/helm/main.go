package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/kubernetes/pkg/util/slice"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/plugins/util/schema"
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

var templateSchema = schema.Schema{
	Type:  "object",
	Title: "HelmAppSpec contains helm app name  in source repo",
	Properties: schema.Properties{
		"name": &schema.Schema{
			Type: "object",
		},
		"parameters": &schema.Schema{
			Type:  "object",
			Title: "HelmParameter is a parameter to a helm template",
			Properties: schema.Properties{
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
			Items: &schema.Schema{
				Type: "string",
			},
		},
		"values": {
			Type:  "string",
			Title: "the contents of values.yaml",
		},
	},
}

func runDiscover(root string) []string {
	apps := make([]string, 0)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(filepath.Base(path), "Chart.yaml") {
			rel, _ := filepath.Rel(root, filepath.Dir(path))
			apps = append(apps, rel)
		}
		return nil
	})
	errors.CheckError(err)
	return apps
}

func runSchema(path string) {
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
			templateSchema.Properties["valueFiles"].Enum = append(templateSchema.Properties["valueFiles"].Enum, n)
		}
	}
	parameters, err := h.GetParameters([]string{"values.yaml"})
	errors.CheckError(err)

	for name, value := range parameters {
		templateSchema.Properties["parameters"].Enum = append(templateSchema.Properties["parameters"].Enum, name)
		templateSchema.Properties["parameters"].Title = fmt.Sprintf("%s, %s=%s", templateSchema.Properties["parameters"].Title, name, value)
	}

	for name := range templateSchema.Properties {
		slice.SortStrings(templateSchema.Properties[name].Enum)
	}

	output, err := json.Marshal(templateSchema)
	errors.CheckError(err)
	fmt.Println(string(output))
}

func runTemplate(path string) {
	bytes, err := ioutil.ReadAll(os.Stdin)
	errors.CheckError(err)
	spec := &Spec{}
	err = yaml.Unmarshal(bytes, spec)
	errors.CheckError(err)
	app, err := helm.NewHelmApp(path, nil)
	errors.CheckError(err)
	set := map[string]string{}
	setString := map[string]string{}
	for _, p := range spec.Parameters {
		if p.ForceString {
			setString[p.Name] = p.Value
		} else {
			set[p.Name] = p.Value
		}
	}
	if spec.Values != "" {
		f, err := ioutil.TempFile(".", "helm")
		defer func() { _ = os.RemoveAll(f.Name()) }()
		errors.CheckError(err)
		err = ioutil.WriteFile(f.Name(), []byte(spec.Values), 777)
		errors.CheckError(err)
	}
	output, err := app.Template(&helm.TemplateOpts{
		Name:        os.Getenv("ARGOCD_APP_NAME"),
		Namespace:   os.Getenv("ARGOCD_APP_NAMESPACE"),
		KubeVersion: os.Getenv("ARGOCD_KUBE_VERSON"),
		Set:         set,
		SetString:   setString,
		Values:      spec.ValueFiles,
	})
	errors.CheckError(err)
	fmt.Println(output)
}

func main() {
	cmd := cobra.Command{
		Use: "helm",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	cmd.AddCommand(&cobra.Command{Use: "discover", Run: func(cmd *cobra.Command, args []string) {
		for _, path := range runDiscover(args[0]) {
			fmt.Println(path)
		}
	}})
	cmd.AddCommand(&cobra.Command{Use: "schema", Run: func(cmd *cobra.Command, args []string) {
		runSchema(args[0])
	}})
	cmd.AddCommand(&cobra.Command{Use: "template", Run: func(cmd *cobra.Command, args []string) {
		runTemplate(args[0])
	}})
	errors.CheckError(cmd.Execute())
}
