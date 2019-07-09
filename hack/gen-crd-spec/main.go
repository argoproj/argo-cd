package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application"
	"github.com/argoproj/argo-cd/util/kube"

	"github.com/ghodss/yaml"
	extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	kindToCRDPath = map[string]string{
		application.ApplicationFullName: "manifests/crds/application-crd.yaml",
		application.AppProjectFullName:  "manifests/crds/appproject-crd.yaml",
	}
)

func getCustomResourceDefinitions() map[string]*extensionsobj.CustomResourceDefinition {
	crdYamlBytes, err := exec.Command(
		"controller-gen",
		"paths=./pkg/apis/application/...",
		"crd:trivialVersions=true",
		"output:crd:stdout",
	).Output()
	checkErr(err)

	// clean up stuff left by controller-gen
	deleteFile("config/webhook/manifests.yaml")
	deleteFile("config/webhook")
	deleteFile("config")

	objs, err := kube.SplitYAML(string(crdYamlBytes))
	checkErr(err)
	crds := make(map[string]*extensionsobj.CustomResourceDefinition)
	for i := range objs {
		un := objs[i]

		// We need to completely remove validation of problematic fields such as creationTimestamp,
		// which get marshalled to `null`, but are typed as as a `string` during Open API validation
		removeValidataion(un, "metadata.creationTimestamp")

		crd := toCRD(un)
		crd.Labels = map[string]string{
			"app.kubernetes.io/name":    crd.Name,
			"app.kubernetes.io/part-of": "argocd",
		}
		crd.Spec.Scope = "Namespaced"
		crds[crd.Name] = crd
	}
	return crds
}

func deleteFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}
	checkErr(os.Remove(path))
}

func removeValidataion(un *unstructured.Unstructured, path string) {
	schemaPath := []string{"spec", "validation", "openAPIV3Schema"}
	for _, part := range strings.Split(path, ".") {
		schemaPath = append(schemaPath, "properties", part)
	}
	unstructured.RemoveNestedField(un.Object, schemaPath...)
}

func toCRD(un *unstructured.Unstructured) *extensionsobj.CustomResourceDefinition {
	unBytes, err := json.Marshal(un)
	checkErr(err)

	var crd extensionsobj.CustomResourceDefinition
	err = json.Unmarshal(unBytes, &crd)
	checkErr(err)

	return &crd
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	crds := getCustomResourceDefinitions()
	for kind, path := range kindToCRDPath {
		crd := crds[kind]
		if crd == nil {
			panic(fmt.Sprintf("CRD of kind %s was not generated", kind))
		}

		jsonBytes, err := json.Marshal(crd)
		checkErr(err)

		var r unstructured.Unstructured
		err = json.Unmarshal(jsonBytes, &r.Object)
		checkErr(err)

		// clean up crd yaml before marshalling
		unstructured.RemoveNestedField(r.Object, "status")
		unstructured.RemoveNestedField(r.Object, "metadata", "creationTimestamp")
		jsonBytes, err = json.MarshalIndent(r.Object, "", "    ")
		checkErr(err)

		yamlBytes, err := yaml.JSONToYAML(jsonBytes)
		checkErr(err)

		err = ioutil.WriteFile(path, yamlBytes, 0644)
		checkErr(err)
	}
}
