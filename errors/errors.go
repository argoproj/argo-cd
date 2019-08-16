package errors

import (
	log "github.com/sirupsen/logrus"
)

// CheckError is a convenience function to exit if an error is non-nil and exit if it was
func CheckError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// panics if there is an error.
// This returns the first value so you can use it if you cast it:
// text := FailOrErr(Foo)).(string)
func FailOnErr(v interface{}, err error) interface{} {
	CheckError(err)
	return v
}
