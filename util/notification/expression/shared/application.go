package shared

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func GetApplication(obj *unstructured.Unstructured) (*v1alpha1.Application, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	application := &v1alpha1.Application{}
	if err := json.Unmarshal(data, application); err != nil {
		return nil, err
	}

	return application, nil
}
