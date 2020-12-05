package stats

import (
	"time"
)

// mock out time.Now() for unit tests
var now = time.Now

// TimingStats is a helper to breakdown the timing of an expensive function call
// Usage:
// ts := NewTimingStats()
// ts.AddCheckpoint("checkpoint-1")
// ...
// ts.AddCheckpoint("checkpoint-2")
// ...
// ts.AddCheckpoint("checkpoint-3")
// ts.Timings()
type TimingStats struct {
	StartTime time.Time

	checkpoints []tsCheckpoint
}

type tsCheckpoint struct {
	name string
	time time.Time
}

func NewTimingStats() *TimingStats {
	return &TimingStats{
		StartTime: now(),
	}
}

func (t *TimingStats) AddCheckpoint(name string) {
	cp := tsCheckpoint{
		name: name,
		time: now(),
	}
	t.checkpoints = append(t.checkpoints, cp)
}

func (t *TimingStats) Timings() map[string]time.Duration {
	timings := make(map[string]time.Duration)
	prev := t.StartTime
	for _, cp := range t.checkpoints {
		timings[cp.name] = cp.time.Sub(prev)
		prev = cp.time
	}
	return timings
}
