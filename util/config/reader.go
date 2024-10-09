package config

import (
	"encoding/json"
	"io"
	"net/http"
	"os"

	"sigs.k8s.io/yaml"
)

// UnmarshalReader is used to read manifests from stdin
func UnmarshalReader(reader io.Reader, obj interface{}) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return unmarshalObject(data, obj)
}

// unmarshalObject tries to convert a YAML or JSON byte array into the provided type.
func unmarshalObject(data []byte, obj interface{}) error {
	// first, try unmarshaling as JSON
	// Based on technique from Kubectl, which supports both YAML and JSON:
	//   https://mlafeldt.github.io/blog/teaching-go-programs-to-love-json-and-yaml/
	//   http://ghodss.com/2014/the-right-way-to-handle-yaml-in-golang/
	// Short version: JSON unmarshaling won't zero out null fields; YAML unmarshaling will.
	// This may have unintended effects or hard-to-catch issues when populating our application object.
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, &obj)
}

// MarshalLocalYAMLFile writes JSON or YAML to a file on disk.
// The caller is responsible for checking error return values.
func MarshalLocalYAMLFile(path string, obj interface{}) error {
	yamlData, err := yaml.Marshal(obj)
	if err == nil {
		err = os.WriteFile(path, yamlData, 0o600)
	}
	return err
}

// UnmarshalLocalFile retrieves JSON or YAML from a file on disk.
// The caller is responsible for checking error return values.
func UnmarshalLocalFile(path string, obj interface{}) error {
	data, err := os.ReadFile(path)
	if err == nil {
		err = unmarshalObject(data, obj)
	}
	return err
}

func Unmarshal(data []byte, obj interface{}) error {
	return unmarshalObject(data, obj)
}

// UnmarshalRemoteFile retrieves JSON or YAML through a GET request.
// The caller is responsible for checking error return values.
func UnmarshalRemoteFile(url string, obj interface{}) error {
	data, err := ReadRemoteFile(url)
	if err == nil {
		err = unmarshalObject(data, obj)
	}
	return err
}

// ReadRemoteFile issues a GET request to retrieve the contents of the specified URL as a byte array.
// The caller is responsible for checking error return values.
func ReadRemoteFile(url string) ([]byte, error) {
	var data []byte
	resp, err := http.Get(url)
	if err == nil {
		defer func() {
			_ = resp.Body.Close()
		}()
		data, err = io.ReadAll(resp.Body)
	}
	return data, err
}
