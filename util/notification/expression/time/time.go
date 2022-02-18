package time

import (
	"time"
)

func NewExprs() map[string]interface{} {
	return map[string]interface{}{
		"Parse": parse,
		"Now":   now,
		"LoadLocation": load_location,
	}
}

func parse(timestamp string) time.Time {
	res, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		panic(err)
	}
	return res
}

func now() time.Time {
	return time.Now()
}

func load_location(location string) time.Location {
	loc, err := time.LoadLocation(location)
	if err != nil {
		panic(err)
	}
	return loc
}
