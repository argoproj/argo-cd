package errors

import (
	log "github.com/sirupsen/logrus"
)

// CheckError is a convenience function to exit if there was error is non-nil
func CheckError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
