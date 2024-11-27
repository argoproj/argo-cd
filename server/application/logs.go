package application

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type logEntry struct {
	line      string
	timeStamp time.Time
	podName   string
	err       error
}

// parseLogsStream converts given ReadCloser into channel that emits log entries
func parseLogsStream(podName string, stream io.ReadCloser, ch chan logEntry) {
	bufReader := bufio.NewReader(stream)
	eof := false
	for !eof {
		line, err := bufReader.ReadString('\n')
		if err != nil && errors.Is(err, io.EOF) {
			eof = true
			// stop if we reached end of stream and the next line is empty
			if line == "" {
				break
			}
		} else if err != nil && !errors.Is(err, io.EOF) {
			ch <- logEntry{err: err}
			break
		}

		line = strings.TrimSpace(line) // Remove trailing line ending
		parts := strings.Split(line, " ")
		timeStampStr := parts[0]
		logTime, err := time.Parse(time.RFC3339Nano, timeStampStr)
		if err != nil {
			ch <- logEntry{err: err}
			break
		}

		lines := strings.Join(parts[1:], " ")
		for _, line := range strings.Split(lines, "\r") {
			ch <- logEntry{line: line, timeStamp: logTime, podName: podName}
		}
	}
}

// mergeLogStreams merge two stream of logs and ensures that merged logs are sorted by timestamp.
// The implementation uses merge sort: method reads next log entry from each stream if one of streams is empty
// it waits for no longer than specified duration and then merges available entries.
func mergeLogStreams(streams []chan logEntry, bufferingDuration time.Duration) chan logEntry {
	merged := make(chan logEntry)

	// buffer of received log entries for each stream
	entriesPerStream := make([][]logEntry, len(streams))
	process := make(chan struct{})

	var lock sync.Mutex
	streamsCount := int32(len(streams))

	// start goroutine per stream that continuously put new log entries into buffer and triggers processing
	for i := range streams {
		go func(index int) {
			for next := range streams[index] {
				lock.Lock()
				entriesPerStream[index] = append(entriesPerStream[index], next)
				lock.Unlock()
				process <- struct{}{}
			}
			// stop processing after all streams got closed
			if atomic.AddInt32(&streamsCount, -1) == 0 {
				close(process)
			}
		}(i)
	}

	// send moves log entries from buffer into merged stream
	// if flush=true then sends log entries into merged stream even if buffer of some streams are empty
	send := func(flush bool) bool {
		var entries []logEntry
		lock.Lock()
		for {
			oldest := -1
			someEmpty := false
			allEmpty := true
			for i := range entriesPerStream {
				entries := entriesPerStream[i]
				if len(entries) > 0 {
					if oldest == -1 || entriesPerStream[oldest][0].timeStamp.After(entries[0].timeStamp) {
						oldest = i
					}
					allEmpty = false
				} else {
					someEmpty = true
				}
			}

			if allEmpty || someEmpty && !flush {
				break
			}

			if oldest > -1 {
				entries = append(entries, entriesPerStream[oldest][0])
				entriesPerStream[oldest] = entriesPerStream[oldest][1:]
			}
		}
		lock.Unlock()
		for i := range entries {
			merged <- entries[i]
		}
		return len(entries) > 0
	}

	var sentAtLock sync.Mutex
	var sentAt time.Time

	ticker := time.NewTicker(bufferingDuration)
	go func() {
		for range ticker.C {
			sentAtLock.Lock()
			// waited long enough for logs from each streams, send everything accumulated
			if sentAt.Add(bufferingDuration).Before(time.Now()) {
				_ = send(true)
				sentAt = time.Now()
			}

			sentAtLock.Unlock()
		}
	}()

	go func() {
		for range process {
			if send(false) {
				sentAtLock.Lock()
				sentAt = time.Now()
				sentAtLock.Unlock()
			}
		}

		_ = send(true)

		ticker.Stop()
		close(merged)
	}()
	return merged
}
