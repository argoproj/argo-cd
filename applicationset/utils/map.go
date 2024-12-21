package utils

import (
	"fmt"
)

func ConvertToMapStringString(mapStringInterface map[string]interface{}) map[string]string {
	mapStringString := make(map[string]string, len(mapStringInterface))

	for key, value := range mapStringInterface {
		mapStringString[key] = fmt.Sprintf("%v", value)
	}
	return mapStringString
}

func ConvertToMapStringInterface(mapStringString map[string]string) map[string]interface{} {
	mapStringInterface := make(map[string]interface{}, len(mapStringString))

	for key, value := range mapStringString {
		mapStringInterface[key] = value
	}
	return mapStringInterface
}

func CombineStringMaps(aSI map[string]interface{}, bSI map[string]interface{}) (map[string]string, error) {
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
func CombineStringMapsAllowDuplicates(aSI map[string]interface{}, bSI map[string]interface{}) (map[string]string, error) {
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

func CombineMaps(firstMap map[string]interface{}, secondMap map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(firstMap))
	for k, v := range firstMap {
		out[k] = v
	}
	for k, v := range secondMap {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = CombineMaps(bv, v)

					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
