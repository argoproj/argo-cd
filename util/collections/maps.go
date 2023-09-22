package collections

import "reflect"

// CopyStringMap creates copy of a string map
func CopyStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

// StringMapsEqual compares two string maps assuming that nil and empty map are considered equal
func StringMapsEqual(first map[string]string, second map[string]string) bool {
	if first == nil {
		first = map[string]string{}
	}
	if second == nil {
		second = map[string]string{}
	}
	return reflect.DeepEqual(first, second)
}

func MergeStringMaps(items ...map[string]string) map[string]string {
	res := make(map[string]string)
	for _, m := range items {
		if m == nil {
			continue
		}
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}
