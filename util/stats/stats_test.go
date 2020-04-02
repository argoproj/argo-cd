package stats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimingStats(t *testing.T) {
	start := time.Now()
	now = func() time.Time {
		return start
	}
	defer func() {
		now = time.Now
	}()
	ts := NewTimingStats()
	now = func() time.Time {
		return start.Add(100 * time.Millisecond)
	}
	ts.AddCheckpoint("checkpoint-1")
	now = func() time.Time {
		return start.Add(300 * time.Millisecond)
	}
	ts.AddCheckpoint("checkpoint-2")
	timings := ts.Timings()
	assert.Len(t, timings, 2)
	assert.Equal(t, 100*time.Millisecond, timings["checkpoint-1"])
	assert.Equal(t, 200*time.Millisecond, timings["checkpoint-2"])
}
