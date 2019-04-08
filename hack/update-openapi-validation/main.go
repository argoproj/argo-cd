package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	crdutil "github.com/ant31/crd-validation/pkg"
	"github.com/ghodss/yaml"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

// Generate OpenAPI spec definitions for Rollout Resource
func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: %s CRDSPEC TYPE", os.Args[0])
	}
	crdPath := os.Args[1]
	typePath := os.Args[2]

	data, err := ioutil.ReadFile(crdPath)
	if err != nil {
		log.Fatal(err)
	}
	var crd apiextensions.CustomResourceDefinition
	err = yaml.Unmarshal(data, &crd)
	if err != nil {
		log.Fatal(err)
	}
	crd.Spec.Validation = crdutil.GetCustomResourceValidation(typePath, v1alpha1.GetOpenAPIDefinitions)

	jsonBytes, err := json.Marshal(crd)
	if err != nil {
		log.Fatal(err)
	}

	var r unstructured.Unstructured
	if err := json.Unmarshal(jsonBytes, &r.Object); err != nil {
		log.Fatal(err)
	}

	unstructured.RemoveNestedField(r.Object, "status")
	unstructured.RemoveNestedField(r.Object, "metadata", "creationTimestamp")
	jsonBytes, err = json.MarshalIndent(r.Object, "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile(crdPath, yamlBytes, 0644)
	if err != nil {
		panic(err)
	}
	os.Exit(0)

}
