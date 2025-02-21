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

func ConvertToMapStringInterface(mapStringString map[string]string) map[string]any {
	mapStringInterface := make(map[string]any, len(mapStringString))

	for key, value := range mapStringString {
		mapStringInterface[key] = value
	}
	return mapStringInterface
}

func CombineStringMaps(aSI map[string]any, bSI map[string]any) (map[string]string, error) {
	a := ConvertToMapStringString(aSI)
	b := ConvertToMapStringString(bSI)

	res := map[string]string{}

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

// CombineStringMapsAllowDuplicates merges two maps. Where there are duplicates, take the latter map's value.
func CombineStringMapsAllowDuplicates(aSI map[string]any, bSI map[string]any) (map[string]string, error) {
	a := ConvertToMapStringString(aSI)
	b := ConvertToMapStringString(bSI)

	res := map[string]string{}

	for k, v := range a {
		res[k] = v
	}

	for k, v := range b {
		res[k] = v
	}

	return res, nil
}

func CombineMaps(firstMap map[string]any, secondMap map[string]any) map[string]any {
	out := make(map[string]any, len(firstMap))
	for k, v := range firstMap {
		out[k] = v
	}
	for k, v := range secondMap {
		if v, ok := v.(map[string]any); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]any); ok {
					out[k] = CombineMaps(bv, v)

					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
