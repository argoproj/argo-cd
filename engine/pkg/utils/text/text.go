package text

func FirstNonEmpty(args ...string) string {
	for _, value := range args {
		if len(value) > 0 {
			return value
		}
	}
	return ""
}
