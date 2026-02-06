package log

import (
	log "github.com/sirupsen/logrus"
)

// used to switch logging to debug level for a single func
func Debug() func() {
	log.SetLevel(log.DebugLevel)
	return func() { log.SetLevel(log.InfoLevel) }
}
