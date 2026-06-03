package application

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLogsStream_Successful(t *testing.T) {
	r := io.NopCloser(strings.NewReader(`2021-02-09T22:13:45.916570818Z hello
2021-02-09T22:13:45.916570818Z world`))

	res := make(chan logEntry)
	go func() {
		parseLogsStream(context.Background(), "test", r, res)
		close(res)
	}()

	var entries []logEntry
	expectedTimestamp, err := time.Parse(time.RFC3339Nano, "2021-02-09T22:13:45.916570818Z")
	require.NoError(t, err)

	for entry := range res {
		entries = append(entries, entry)
	}

	assert.Equal(t, []logEntry{
		{timeStamp: expectedTimestamp, podName: "test", line: "hello"},
		{timeStamp: expectedTimestamp, podName: "test", line: "world"},
	}, entries)
}

func TestParseLogsStream_ParsingError(t *testing.T) {
	r := io.NopCloser(strings.NewReader(`hello world`))

	res := make(chan logEntry)
	go func() {
		parseLogsStream(context.Background(), "test", r, res)
		close(res)
	}()

	var entries []logEntry
	for entry := range res {
		entries = append(entries, entry)
	}

	require.Len(t, entries, 1)
	assert.Error(t, entries[0].err)
}

func TestMergeLogStreams(t *testing.T) {
	first := make(chan logEntry)
	go func() {
		parseLogsStream(context.Background(), "first", io.NopCloser(strings.NewReader(`2021-02-09T00:00:01Z 1
2021-02-09T00:00:03Z 3`)), first)
		close(first)
	}()

	second := make(chan logEntry)
	go func() {
		parseLogsStream(context.Background(), "second", io.NopCloser(strings.NewReader(`2021-02-09T00:00:02Z 2
2021-02-09T00:00:04Z 4`)), second)
		close(second)
	}()

	merged := mergeLogStreams(context.Background(), []chan logEntry{first, second}, time.Second)
	var lines []string
	for entry := range merged {
		lines = append(lines, entry.line)
	}

	assert.Equal(t, []string{"1", "2", "3", "4"}, lines)
}

func TestMergeLogStreams_RaceCondition(_ *testing.T) {
	// Test for regression of this issue: https://github.com/argoproj/argo-cd/issues/7006
	for i := range 5000 {
		first := make(chan logEntry)
		second := make(chan logEntry)

		go func() {
			parseLogsStream(context.Background(), "first", io.NopCloser(strings.NewReader(`2021-02-09T00:00:01Z 1`)), first)
			time.Sleep(time.Duration(i%3) * time.Millisecond)
			close(first)
		}()

		go func() {
			parseLogsStream(context.Background(), "second", io.NopCloser(strings.NewReader(`2021-02-09T00:00:02Z 2`)), second)
			time.Sleep(time.Duration((i+1)%3) * time.Millisecond)
			close(second)
		}()

		merged := mergeLogStreams(context.Background(), []chan logEntry{first, second}, 1*time.Millisecond)

		// Drain the channel
		for range merged {
		}

		// This test intentionally doesn't test the order of the output. Under these intense conditions, the test would
		// fail often due to out of order entries. This test is only meant to reproduce a race between a channel writer
		// and channel closer.
	}
}

// TestMergeLogStreams_ContextCancellation verifies that cancelling the context causes mergeLogStreams
// to close the merged channel promptly, allowing all internal goroutines to exit without leaking.
func TestMergeLogStreams_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// unbuffered pipe: write end will block until someone reads
	pr, pw := io.Pipe()

	ch := make(chan logEntry)
	go func() {
		parseLogsStream(ctx, "test", pr, ch)
		close(ch)
	}()

	merged := mergeLogStreams(ctx, []chan logEntry{ch}, time.Second)

	// cancel before the pipe produces any data
	cancel()
	_ = pw.Close()

	// merged must be closed (context cancelled), not block forever
	done := make(chan struct{})
	go func() {
		for range merged {
		}
		close(done)
	}()

	select {
	case <-done:
		// merged closed promptly — no leak
	case <-time.After(5 * time.Second):
		t.Fatal("mergeLogStreams did not close merged channel after context cancellation")
	}
}
