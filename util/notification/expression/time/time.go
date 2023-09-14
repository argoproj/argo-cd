package time

import (
	"time"
)

func NewExprs() map[string]interface{} {
	return map[string]interface{}{
		"Parse":  parse,
		"Now":    now,
		"Format": format,
	}
}

func parse(timestamp string) time.Time {
	res, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		panic(err)
	}
	return res
}

func format(t time.Time) string {
	return t.Format(time.RFC3339)
}

func now() time.Time {
	return time.Now()
}
