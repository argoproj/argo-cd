package utils

import (
	"fmt"
)

func ConvertToMapStringString(mapStringInterface map[string]any) map[string]string {
	mapStringString := make(map[string]string, len(mapStringInterface))

	for key, value := range mapStringInterface {
		mapStringString[key] = fmt.Sprintf("%v", value)
	}
	return mapStringString
}

func CombineStringMaps(aSI map[string]any, bSI map[string]any) (map[string]any, error) {
	a := ConvertToMapStringString(aSI)
	b := ConvertToMapStringString(bSI)

	res := map[string]any{}

	for k, v := range a {
		res[k] = v
	}

	for k, v := range b {
		current, present := res[k]
		if present && current != v {
			return nil, fmt.Errorf("found duplicate key %s with different value, a: %s ,b: %s", k, current, v)
		}
		res[k] = v
	}

	return res, nil
}
