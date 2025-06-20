package normalizers

import (
	"fmt"
	"strings"
)

func getGroupKindForOverrideKey(key string) (string, string, error) {
	var group, kind string
	parts := strings.Split(key, "/")

	switch len(parts) {
	case 2:
		group = parts[0]
		kind = parts[1]
	case 1:
		kind = parts[0]
	default:
		return "", "", fmt.Errorf("override key must be <group>/<kind> or <kind>, got: '%s' ", key)
	}
	return group, kind, nil
}
