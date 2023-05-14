package generators

import (
	"fmt"
	"strconv"
)

func flattenParameters(in map[string]interface{}) map[string]string {
	out := make(map[string]string)

	for key, value := range in {
		// Handle string values
		if v, ok := value.(string); ok {
			out[key] = v
			continue
		}

		// Handle integer values
		if v, ok := value.(int); ok {
			out[key] = strconv.Itoa(v)
			continue
		}

		var children map[string]interface{}
		switch val := value.(type) {
		case map[string]interface{}:
			children = val

		case map[string]string:
			children = make(map[string]interface{}, len(val))
			for k, v := range val {
				children[k] = v
			}

		case []interface{}:
			children = make(map[string]interface{}, len(val))
			for i, x := range val {
				children[fmt.Sprintf("%d", i)] = fmt.Sprintf("%v", x)
			}

		case []string:
			children = make(map[string]interface{}, len(val))
			for i, x := range val {
				children[fmt.Sprintf("%d", i)] = x
			}
		}

		for k, v := range flattenParameters(children) {
			out[fmt.Sprintf("%s.%s", key, k)] = v
		}
	}

	return out
}
