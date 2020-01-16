package testing

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UnstructuredFromFile(path string) *unstructured.Unstructured {
	file, err := ioutil.ReadFile(path)
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
