package application

import (
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
		parseLogsStream("test", r, res)
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
		parseLogsStream("test", r, res)
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
		parseLogsStream("first", io.NopCloser(strings.NewReader(`2021-02-09T00:00:01Z 1
2021-02-09T00:00:03Z 3`)), first)
		close(first)
	}()

	second := make(chan logEntry)
	go func() {
		parseLogsStream("second", io.NopCloser(strings.NewReader(`2021-02-09T00:00:02Z 2
2021-02-09T00:00:04Z 4`)), second)
		close(second)
	}()

	merged := mergeLogStreams([]chan logEntry{first, second}, time.Second)
	var lines []string
	for entry := range merged {
		lines = append(lines, entry.line)
	}

	assert.Equal(t, []string{"1", "2", "3", "4"}, lines)
}
