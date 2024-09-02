package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	extensionsobj "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

var kindToCRDPath = map[string]string{
	application.ApplicationFullName:    "manifests/crds/application-crd.yaml",
	application.AppProjectFullName:     "manifests/crds/appproject-crd.yaml",
	application.ApplicationSetFullName: "manifests/crds/applicationset-crd.yaml",
}

func getCustomResourceDefinitions() map[string]*extensionsobj.CustomResourceDefinition {
	crdYamlBytes, err := exec.Command(
		"controller-gen",
		"paths=./pkg/apis/application/...",
		"crd:crdVersions=v1",
		"output:crd:stdout",
	).Output()
	checkErr(err)

	// clean up stuff left by controller-gen
	deleteFile("config/webhook/manifests.yaml")
	deleteFile("config/webhook")
	deleteFile("config/argoproj.io_applications.yaml")
	deleteFile("config/argoproj.io_appprojects.yaml")
	deleteFile("config/argoproj.io_applicationsets.yaml")
	deleteFile("config")

	objs, err := kube.SplitYAML(crdYamlBytes)
	checkErr(err)
	crds := make(map[string]*extensionsobj.CustomResourceDefinition)
	for i := range objs {
		un := objs[i]

		// We need to completely remove validation of problematic fields such as creationTimestamp,
		// which get marshalled to `null`, but are typed as as a `string` during Open API validation
		removeValidation(un, "metadata.creationTimestamp")
		// remove status validation for AppProject CRD as workaround for https://github.com/argoproj/argo-cd/issues/4158
		if un.GetName() == "appprojects.argoproj.io" {
			removeValidation(un, "status")
		}

		crd := toCRD(un, un.GetName() == "applicationsets.argoproj.io")
		crd.Labels = map[string]string{
			"app.kubernetes.io/name":    crd.Name,
			"app.kubernetes.io/part-of": "argocd",
		}
		delete(crd.Annotations, "controller-gen.kubebuilder.io/version")
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

func removeValidation(un *unstructured.Unstructured, path string) {
	schemaPath := []string{"spec", "versions[*]", "schema", "openAPIV3Schema"}
	for _, part := range strings.Split(path, ".") {
		schemaPath = append(schemaPath, "properties", part)
	}
	unstructured.RemoveNestedField(un.Object, schemaPath...)
}

func toCRD(un *unstructured.Unstructured, removeDesc bool) *extensionsobj.CustomResourceDefinition {
	if removeDesc {
		removeDescription(un.Object)
	}
	unBytes, err := json.Marshal(un)
	checkErr(err)

	var crd extensionsobj.CustomResourceDefinition
	err = json.Unmarshal(unBytes, &crd)
	checkErr(err)

	return &crd
}

func removeDescription(v interface{}) {
	switch v := v.(type) {
	case []interface{}:
		for _, v := range v {
			removeDescription(v)
		}
	case map[string]interface{}:
		if _, ok := v["description"]; ok {
			_, ok := v["description"].(string)
			if ok {
				delete(v, "description")
			}
		}
		for _, v := range v {
			removeDescription(v)
		}
	}
}

func checkErr(err error) {
	if err != nil {
		var execError *exec.ExitError
		if errors.As(err, &execError) {
			fmt.Println(string(execError.Stderr))
		}
		panic(err)
	}
}

func main() {
	crdsapp := getCustomResourceDefinitions()
	for kind, path := range kindToCRDPath {
		crd := crdsapp[kind]
		if crd == nil {
			panic(fmt.Sprintf("CRD of kind %s was not generated", kind))
		}
		writeCRDintoFile(crd, path)
	}
}

func writeCRDintoFile(crd *extensionsobj.CustomResourceDefinition, path string) {
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

	err = os.WriteFile(path, yamlBytes, 0o644)
	checkErr(err)
}
