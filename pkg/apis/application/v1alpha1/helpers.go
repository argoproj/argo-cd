package v1alpha1

import (
	"encoding/json"
	"fmt"
)

func TruncateByDepth(raw []byte, maxDepth int64) ([]byte, error) {
	var obj interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json: %w", err)
	}

	truncated := truncateRecursively(obj, 0, maxDepth)

	result, err := json.Marshal(truncated)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to json: %w", err)
	}

	return result, nil
}

func truncateRecursively(val any, depth, maxDepth int64) interface{} {
	if depth >= maxDepth {
		switch val.(type) {
		case map[string]any, []any:
			return "...(truncated)"
		default:
			return val
		}
	}

	switch v := val.(type) {
	case map[string]any:
		m := map[string]any{}
		for k, child := range v {
			m[k] = truncateRecursively(child, depth+1, maxDepth)
		}
		return m
	case []interface{}:
		arr := make([]any, len(v))
		for i, child := range v {
			arr[i] = truncateRecursively(child, depth+1, maxDepth)
		}
		return arr
	default:
		return v
	}
}
