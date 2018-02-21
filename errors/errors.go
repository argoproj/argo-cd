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
