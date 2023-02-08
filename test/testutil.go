package test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-openapi/jsonpointer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// StartInformer is a helper to start an informer, wait for its cache to sync and return a cancel func
func StartInformer(informer cache.SharedIndexInformer) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		log.Fatal("Timed out waiting for informer cache to sync")
	}
	return cancel
}

// GetFreePort finds an available free port on the OS
func GetFreePort() (int, error) {
	ln, err := net.Listen("tcp", "[::]:0")
	if err != nil {
		return 0, err
	}
	return ln.Addr().(*net.TCPAddr).Port, ln.Close()
}

// WaitForPortListen waits until the given address is listening on the port
func WaitForPortListen(addr string, timeout time.Duration) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(timeout)
	if timeout == 0 {
		timer.Stop()
	} else {
		defer timer.Stop()
	}
	for {
		select {
		case <-ticker.C:
			if portIsOpen(addr) {
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("timeout after %s", timeout.String())
		}
	}
}

func portIsOpen(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Read the contents of a file and returns it as string. Panics on error.
func MustLoadFileToString(path string) string {
	o, err := os.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	return string(o)
}

func YamlToUnstructured(yamlStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yamlStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

// ToMap converts any object to a map[string]interface{}
func ToMap(obj interface{}) map[string]interface{} {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	var res map[string]interface{}
	err = json.Unmarshal(data, &res)
	if err != nil {
		panic(err)
	}
	return res
}

// GetTestDir will return the full directory path of the
// calling test file.
func GetTestDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return cwd
}

// Convert an object into apiextensionsv1.JSON Panic if not possible
func ToApiExtenstionsJSON(object interface{}) apiextensionsv1.JSON {
	return apiextensionsv1.JSON{
		Raw: []byte(ToJSON(object)),
	}
}

// Convert an object into string representation Json. Panic if not possible
func ToJSON(object interface{}) string {
	output, err := json.Marshal(object)

	if err != nil {
		panic(err)
	}

	return string(output)
}

// Update object in apiextensionsv1.JSON string using json pointer
func UpdateDataAsJson(jsonString apiextensionsv1.JSON, jsonPointer string, value interface{}) apiextensionsv1.JSON {
	return apiextensionsv1.JSON{
		Raw: []byte(UpdateDataAsJsonString(string(jsonString.Raw), jsonPointer, value)),
	}
}

// Update object in json string using json pointer
func UpdateDataAsJsonString(jsonString string, jsonPointer string, value interface{}) string {
	var object map[string]interface{}

	err := json.Unmarshal([]byte(jsonString), &object)

	if err != nil {
		panic(err)
	}

	data := UpdateData(object, jsonPointer, value)
	return ToJSON(data)
}

// Update object field using json pointer
func UpdateData(object interface{}, jsonPointer string, value interface{}) interface{} {

	pointer, err := jsonpointer.New(jsonPointer)

	if err != nil {
		panic(err)
	}

	output, err := pointer.Set(object, value)

	if err != nil {
		panic(err)
	}

	return output
}

// Get object field using json pointer
func GetData(object interface{}, jsonPointer string) interface{} {

	pointer, err := jsonpointer.New(jsonPointer)

	if err != nil {
		panic(err)
	}

	output, _, err := pointer.Get(object)

	if err != nil {
		panic(err)
	}

	return output
}
