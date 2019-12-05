package tracing

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

/*
	Poor Mans OpenTracing.

	Standardizes logging of operation duration.
*/

var enabled = false
var logger = log.New()

func init() {
	enabled = os.Getenv("ARGOCD_TRACING_ENABLED") == "1"
}

type Span struct {
	operationName string
	baggage       map[string]interface{}
	start         time.Time
}

func (s Span) Finish() {
	if enabled {
		logger.WithFields(s.baggage).
			WithField("operation_name", s.operationName).
			WithField("time_ms", time.Since(s.start).Seconds()*1e3).
			Info()
	}
}

func (s Span) SetBaggageItem(key string, value interface{}) {
	s.baggage[key] = value
}

func StartSpan(operationName string) Span {
	return Span{operationName, make(map[string]interface{}), time.Now()}
}
