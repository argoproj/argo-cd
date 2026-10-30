package hydrator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

var sprigFuncMap = sprig.GenericFuncMap() // a singleton for better performance

func init() {
	// Avoid allowing the user to learn things about the environment.
	delete(sprigFuncMap, "env")
	delete(sprigFuncMap, "expandenv")
	delete(sprigFuncMap, "getHostByName")
}

// Render use a parsed template and calls the Execute to apply the data.
// currently the method supports struct and a map[string]any as data
func Render(tmpl string, data HydratorCommitMetadata) (string, error) {
	var dataMap map[string]any
	var err error
	// short-circuit if template is not defined
	if tmpl == "" {
		return "", nil
	}
	dataMap, err = structToMap(data)
	if err != nil {
		return "", fmt.Errorf("marshaling failed: %w", err)
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
