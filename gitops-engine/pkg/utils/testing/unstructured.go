package testing

import (
	"encoding/json"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func UnstructuredFromFile(path string) *unstructured.Unstructured {
	file, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return Unstructured(string(file))
}

func Unstructured(text string) *unstructured.Unstructured {
	un := &unstructured.Unstructured{}
	var err error
	if strings.HasPrefix(text, "{") {
		err = json.Unmarshal([]byte(text), &un)
	} else {
		err = yaml.Unmarshal([]byte(text), &un)
	}
	if err != nil {
		panic(err)
	}
	return un
}
