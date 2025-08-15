package hydrator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

var sprigFuncMap = sprig.GenericFuncMap() // a singleton for better performance

func init() {
	// Avoid allowing the user to learn things about the environment.
	delete(sprigFuncMap, "env")
	delete(sprigFuncMap, "expandenv")
	delete(sprigFuncMap, "getHostByName")
	sprigFuncMap["kindIs"] = kindIs
}

// Render use a parsed template and calls the Execute to apply the data.
// currently the method supports struct and a map[string]any as data
func Render(tmpl string, data any) (string, error) {
	var dataMap map[string]any
	var err error
	t := reflect.TypeOf(data)
	switch t.Kind() {
	case reflect.Map:
		if m, ok := data.(map[string]any); ok {
			dataMap = m
		} else {
			return "", fmt.Errorf("failed to cast map type")
		}
	case reflect.Struct:
		dataMap, err = structToMap(data)
		if err != nil {
			return "", fmt.Errorf("marshaling failed: %w", err)
		}
	default:
		return "", fmt.Errorf("type not supported: %T\n", t)
	}

	metadata := map[string]any{
		"metadata": dataMap,
	}
	template, err := template.New("commit-template").Funcs(sprigFuncMap).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", tmpl, err)
	}
	var replacedTmplBuffer bytes.Buffer
	if err = template.Execute(&replacedTmplBuffer, metadata); err != nil {
		return "", fmt.Errorf("failed to execute go template %s: %w", tmpl, err)
	}

	return replacedTmplBuffer.String(), nil
}

func structToMap(s any) (map[string]any, error) {
	jsonOut, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	err = json.Unmarshal(jsonOut, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func kindIs(kind string, value any) bool {
	switch kind {
	case "map":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	default:
		return false
	}
}
