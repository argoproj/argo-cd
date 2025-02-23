package text

func FirstNonEmpty(args ...string) string {
	for _, value := range args {
		if len(value) > 0 {
			return value
		}
	}
	return ""
}

// WithDefault return defaultValue when val is blank
func WithDefault(val string, defaultValue string) string {
	if len(val) == 0 {
		return defaultValue
	}
	return val
}
