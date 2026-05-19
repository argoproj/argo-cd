package application

import (
	"bufio"
	"context"
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

// parseLogsStream converts given ReadCloser into channel that emits log entries.
// It stops early if ctx is cancelled, avoiding goroutine leaks when the caller disconnects.
func parseLogsStream(ctx context.Context, podName string, stream io.ReadCloser, ch chan logEntry) {
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
			select {
			case ch <- logEntry{err: err}:
			case <-ctx.Done():
			}
			break
		}

		line = strings.TrimSpace(line) // Remove trailing line ending
		parts := strings.Split(line, " ")
		timeStampStr := parts[0]
		logTime, err := time.Parse(time.RFC3339Nano, timeStampStr)
		if err != nil {
			select {
			case ch <- logEntry{err: err}:
			case <-ctx.Done():
			}
			break
		}

		lines := strings.Join(parts[1:], " ")
		for line := range strings.SplitSeq(lines, "\r") {
			select {
			case ch <- logEntry{line: line, timeStamp: logTime, podName: podName}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// mergeLogStreams merge two stream of logs and ensures that merged logs are sorted by timestamp.
// The implementation uses merge sort: method reads next log entry from each stream if one of streams is empty
// it waits for no longer than specified duration and then merges available entries.
// ctx cancellation causes all internal goroutines to exit promptly, preventing goroutine and memory leaks.
func mergeLogStreams(ctx context.Context, streams []chan logEntry, bufferingDuration time.Duration) chan logEntry {
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
				select {
				case process <- struct{}{}:
				case <-ctx.Done():
					// drain remaining entries so parseLogsStream goroutine can exit
					for range streams[index] {
					}
					if atomic.AddInt32(&streamsCount, -1) == 0 {
						close(process)
					}
					return
				}
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
			select {
			case merged <- entries[i]:
			case <-ctx.Done():
				return false
			}
		}
		return len(entries) > 0
	}

	var sentAtLock sync.Mutex
	var sentAt time.Time

	ticker := time.NewTicker(bufferingDuration)
	tickerDone := make(chan struct{})
	go func() {
		for {
			select {
			case <-tickerDone:
				return
			case <-ticker.C:
				sentAtLock.Lock()
				// waited long enough for logs from each streams, send everything accumulated
				if sentAt.Add(bufferingDuration).Before(time.Now()) {
					_ = send(true)
					sentAt = time.Now()
				}
				sentAtLock.Unlock()
			}
		}
	}()

	go func() {
	loop:
		for {
			select {
			case _, ok := <-process:
				if !ok {
					break loop
				}
				if send(false) {
					sentAtLock.Lock()
					sentAt = time.Now()
					sentAtLock.Unlock()
				}
			case <-ctx.Done():
				// client disconnected: stop immediately without flushing
				ticker.Stop()
				tickerDone <- struct{}{}
				close(merged)
				return
			}
		}

		_ = send(true)

		ticker.Stop()
		// ticker.Stop() does not close the channel, and it does not wait for the channel to be drained. So we need to
		// explicitly prevent the goroutine from leaking by closing the channel. We also need to prevent the goroutine
		// from calling `send` again, because `send` pushes to the `merged` channel which we're about to close.
		// This describes the approach nicely: https://stackoverflow.com/questions/17797754/ticker-stop-behaviour-in-golang
		tickerDone <- struct{}{}
		close(merged)
	}()
	return merged
}
