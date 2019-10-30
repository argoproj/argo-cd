package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/plugins/util/exec"
	"github.com/argoproj/argo-cd/plugins/util/schema"
)

type Spec struct {
	NamePrefix   string
	NameSuffix   string
	Images       []string
	CommonLabels map[string]string
}

var templateSchema = schema.Schema{
	Type:  "object",
	Title: "Kustomize app spec",
	Properties: schema.Properties{
		"namePrefix": &schema.Schema{
			Type:  "string",
			Title: "The image name prefix",
		},
		"nameSuffix": &schema.Schema{
			Type:  "string",
			Title: "The image name suffix",
		},
		"images": &schema.Schema{
			Type:  "array",
			Title: "The images",
			Items: &schema.Schema{
				Type: "string",
			},
		},
		"commonLabels": &schema.Schema{
			Type:  "object",
			Title: "Common labels",
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
		if info.Name() == "kustomization.yaml" || info.Name() == "kustomization.yml" || info.Name() == "Kustomization" {
			rel, _ := filepath.Rel(root, filepath.Dir(path))
			apps = append(apps, rel)
		}
		return nil
	})
	errors.CheckError(err)
	return apps
}
func runSchema(path string) {
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
	if spec.NamePrefix != "" {
		exec.Run(path, "kustomize", "edit", "set", "nameprefix", "--", spec.NamePrefix)
	}
	if spec.NameSuffix != "" {
		exec.Run(path, "kustomize", "edit", "set", "namesuffix", "--", spec.NameSuffix)
	}
	for _, imageName := range spec.Images {
		exec.Run(path, "kustomize", "edit", "set", "image", "--", imageName)
	}
	for k, v := range spec.CommonLabels {
		exec.Run(path, "kustomize", "edit", "set", "label", "--", k+":"+v)
	}
	output := exec.Run(path, "kustomize", "build", ".")
	fmt.Println(output)
}

func main() {
	cmd := cobra.Command{
		Use: "kustomize",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use: "discover",
		Run: func(cmd *cobra.Command, args []string) {
			for _, path := range runDiscover(args[0]) {
				fmt.Println(path)
			}
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "schema",
		Run: func(cmd *cobra.Command, args []string) {
			runSchema(args[0])
		},
	})
	cmd.AddCommand(&cobra.Command{Use: "template", Run: func(cmd *cobra.Command, args []string) {
		runTemplate(args[0])
	}})
	err := cmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
