package normalizers

import (
	"fmt"
	"strings"
)

func getGroupKindForOverrideKey(key string) (string, string, error) {
	var group, kind string
	parts := strings.Split(key, "/")

	if len(parts) == 2 {
		group = parts[0]
		kind = parts[1]
	} else if len(parts) == 1 {
		kind = parts[0]
	} else {
		return "", "", fmt.Errorf("override key must be <group>/<kind> or <kind>, got: '%s' ", key)
	}
	return group, kind, nil
}
