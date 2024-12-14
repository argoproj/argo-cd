package utils

import (
	"fmt"
)

func ConvertToMapStringString(mapStringInterface map[string]interface{}) map[string]string {
	mapStringString := make(map[string]string, len(mapStringInterface))

	for key, value := range mapStringInterface {
		strKey := fmt.Sprintf("%v", key)
		strValue := fmt.Sprintf("%v", value)

		mapStringString[strKey] = strValue
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
