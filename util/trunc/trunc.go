package trunc

// truncates messages to 40 characters
func Trunc(message string, n int) string {
	if len(message) > n {
		return message[0:n-3] + "..."
	}
	return message
}
