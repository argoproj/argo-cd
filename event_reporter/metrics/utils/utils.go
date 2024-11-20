package metrics_utils

import (
	"time"
)

type MetricTimer struct {
	startAt time.Time
}

func NewMetricTimer() *MetricTimer {
	return &MetricTimer{
		startAt: time.Now(),
	}
}

func (m *MetricTimer) Duration() time.Duration {
	return time.Since(m.startAt)
}
