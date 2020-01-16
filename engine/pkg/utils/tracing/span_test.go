package tracing

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestStartSpan(t *testing.T) {

	testLogger, hook := test.NewNullLogger()
	defer hook.Reset()
	logger = testLogger
	defer func() { logger = log.New() }()

	t.Run("Disabled", func(t *testing.T) {
		span := StartSpan("my-operation")
		span.SetBaggageItem("my-key", "my-value")
		span.Finish()

		assert.Empty(t, hook.Entries)

	})
	hook.Reset()
	t.Run("Enabled", func(t *testing.T) {
		enabled = true
		defer func() { enabled = false }()
		span := StartSpan("my-operation")
		span.SetBaggageItem("my-key", "my-value")
		span.Finish()

		e := hook.LastEntry()
		if assert.NotNil(t, e) {
			assert.Empty(t, e.Message)
			assert.Equal(t, "my-operation", e.Data["operation_name"])
			assert.Equal(t, "my-value", e.Data["my-key"])
			assert.Contains(t, e.Data, "time_ms")
		}
	})
}
