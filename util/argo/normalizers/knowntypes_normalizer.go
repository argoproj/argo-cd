package normalizers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/diff"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//go:generate go run github.com/argoproj/argo-cd/hack/known_types corev1 k8s.io/api/core/v1 corev1_known_types.go --docs diffing_known_types.txt
var knownTypes = map[string]func() interface{}{}

type knownTypeField struct {
	fieldPath  []string
	newFieldFn func() interface{}
}

type knownTypesNormalizer struct {
	typeFields map[schema.GroupKind][]knownTypeField
}

// NewKnownTypesNormalizer create a normalizer that re-format custom resource fields using built-in Kubernetes types.
func NewKnownTypesNormalizer(overrides map[string]v1alpha1.ResourceOverride) (diff.Normalizer, error) {
	normalizer := knownTypesNormalizer{typeFields: map[schema.GroupKind][]knownTypeField{}}
	for key, override := range overrides {
		parts := strings.Split(key, "/")
		if len(parts) < 2 {
			continue
		}
		gk := schema.GroupKind{Group: parts[0], Kind: parts[1]}
		for _, f := range override.KnownTypeFields {
			if err := normalizer.addKnownField(gk, f.Field, f.Type); err != nil {
				log.Warnf("Failed to configure known field normalizer: %v", err)
			}
		}
	}
	return &normalizer, nil
}

func (n *knownTypesNormalizer) addKnownField(gk schema.GroupKind, fieldPath string, typePath string) error {
	newFieldFn, ok := knownTypes[typePath]
	if !ok {
		return fmt.Errorf("type '%s' is not supported", typePath)
	}
	n.typeFields[gk] = append(n.typeFields[gk], knownTypeField{
		fieldPath:  strings.Split(fieldPath, "."),
		newFieldFn: newFieldFn,
	})
	return nil
}

func normalize(obj map[string]interface{}, field knownTypeField, fieldPath []string) error {
	for i := range fieldPath {
		if nestedField, ok, err := unstructured.NestedFieldNoCopy(obj, fieldPath[:i+1]...); err == nil && ok {
			items, ok := nestedField.([]interface{})
			if !ok {
				continue
			}
			for j := range items {
				item, ok := items[j].(map[string]interface{})
				if !ok {
					continue
				}

				subPath := fieldPath[i+1:]
				if len(subPath) == 0 {
					newItem, err := remarshal(item, field)
					if err != nil {
						return err
					}
					items[j] = newItem
				} else {
					if err = normalize(item, field, subPath); err != nil {
						return err
					}
				}
			}
			return unstructured.SetNestedSlice(obj, items, fieldPath[:i+1]...)
		}
	}

	if fieldVal, ok, err := unstructured.NestedMap(obj, fieldPath...); ok && err == nil {
		newFieldVal, err := remarshal(fieldVal, field)
		if err != nil {
			return err
		}
		err = unstructured.SetNestedField(obj, newFieldVal, fieldPath...)
		if err != nil {
			return err
		}
	}

	return nil
}

func remarshal(fieldVal map[string]interface{}, field knownTypeField) (map[string]interface{}, error) {
	data, err := json.Marshal(fieldVal)
	if err != nil {
		return nil, err
	}
	typedValue := field.newFieldFn()
	err = json.Unmarshal(data, typedValue)
	if err != nil {
		return nil, err
	}
	data, err = json.Marshal(typedValue)
	if err != nil {
		return nil, err
	}
	newFieldVal := map[string]interface{}{}
	err = json.Unmarshal(data, &newFieldVal)
	if err != nil {
		return nil, err
	}
	return newFieldVal, nil
}

// Normalize re-format custom resource fields using built-in Kubernetes types JSON marshaler.
// This technique allows avoiding false drift detections in CRDs that import data structures from Kubernetes codebase.
func (n *knownTypesNormalizer) Normalize(un *unstructured.Unstructured) error {
	if fields, ok := n.typeFields[un.GroupVersionKind().GroupKind()]; ok {
		for _, field := range fields {
			err := normalize(un.Object, field, field.fieldPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
