package text

func FirstNonEmpty(args ...string) string {
	for _, value := range args {
		if value != "" {
			return value
		}
	}
	return ""
}

// WithDefault return defaultValue when val is blank
func WithDefault(val string, defaultValue string) string {
	if val == "" {
		return defaultValue
	}
	return val
}
