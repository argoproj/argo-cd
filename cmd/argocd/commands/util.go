package commands

import (
	"encoding/json"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

// unmarshalApplication tries to convert a YAML or JSON byte array into an Application struct.
func unmarshalApplication(data []byte, app *argoappv1.Application) {
	// first, try unmarshaling as JSON
	// Based on technique from Kubectl, which supports both YAML and JSON:
	//   https://mlafeldt.github.io/blog/teaching-go-programs-to-love-json-and-yaml/
	//   http://ghodss.com/2014/the-right-way-to-handle-yaml-in-golang/
	// Short version: JSON unmarshaling won't zero out null fields; YAML unmarshaling will.
	// This may have unintended effects or hard-to-catch issues when populating our application object.
	data, err := yaml.YAMLToJSON(data)
	if err != nil {
		log.Fatal("Could not decode valid JSON or YAML Kubernetes manifest")
	}
	err = json.Unmarshal(data, &app)
	if err != nil {
		log.Fatalf("Could not unmarshal Kubernetes manifest: %s", string(data))
	}
}

// readLocalFile reads a file from disk and returns its contents as a byte array.
// The caller is responsible for checking error return values.
func readLocalFile(path string) (data []byte, err error) {
	data, err = ioutil.ReadFile(path)
	return
}

// readRemoteFile issues a GET request to retrieve the contents of the specified URL as a byte array.
// The caller is responsible for checking error return values.
func readRemoteFile(url string) (data []byte, err error) {
	resp, err := http.Get(url)
	if err == nil {
		defer func() {
			_ = resp.Body.Close()
		}()
		data, err = ioutil.ReadAll(resp.Body)
	}
	return
}
