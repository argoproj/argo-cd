package helm

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexRecordLockSerializesWriters(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "aaaaaaaa.json")
	var concurrent, maxConcurrent int
	var mu sync.Mutex

	done := make(chan struct{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			unlock, err := acquireIndexLock(t.Context(), record, false, true)
			require.NoError(t, err)
			mu.Lock()
			concurrent++
			if concurrent > maxConcurrent {
				maxConcurrent = concurrent
			}
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			concurrent--
			mu.Unlock()
			unlock()
		})
	}
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("writers deadlocked")
	}
	assert.Equal(t, 1, maxConcurrent, "exclusive lock must serialize writers")
}

func TestIndexRecordLockSharedAllowsReaders(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "bbbbbbbb.json")
	unlock1, err := acquireIndexLock(t.Context(), record, true, true)
	require.NoError(t, err)
	unlock2, err := acquireIndexLock(t.Context(), record, true, true)
	require.NoError(t, err)
	unlock1()
	unlock2()
}

func TestReadIndexRecordStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "cccccccc.json")
	require.NoError(t, os.WriteFile(record, []byte("{}"), 0o600))

	_, err := readIndexRecordState(record)
	require.Error(t, err, "missing state file must read as invalid")

	// Invalidation writes the '!' marker; readers reject the record.
	require.NoError(t, writeIndexRecordState(record, ""))
	_, err = readIndexRecordState(record)
	require.Error(t, err, "invalidated state must reject reads")

	require.NoError(t, writeIndexRecordState(record, stringsRepeat("f", 64)))
	digest, err := readIndexRecordState(record)
	require.NoError(t, err)
	assert.Equal(t, stringsRepeat("f", 64), digest)
}

func stringsRepeat(s string, n int) string {
	out := make([]rune, 0, n*len(s))
	for range n {
		out = append(out, []rune(s)...)
	}
	return string(out)
}

func TestWriteIndexRecordStateInvalidateThenCommit(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "dddddddd.json")
	require.NoError(t, os.WriteFile(record, []byte("{}"), 0o600))

	require.NoError(t, writeIndexRecordState(record, stringsRepeat("a", 64)))
	digest, err := readIndexRecordState(record)
	require.NoError(t, err)
	assert.Equal(t, stringsRepeat("a", 64), digest)

	// Invalidate: readers must reject the record until it is committed again.
	require.NoError(t, writeIndexRecordState(record, ""))
	_, err = readIndexRecordState(record)
	require.Error(t, err, "invalidated state must reject reads")

	require.NoError(t, writeIndexRecordState(record, stringsRepeat("b", 64)))
	digest, err = readIndexRecordState(record)
	require.NoError(t, err)
	assert.Equal(t, stringsRepeat("b", 64), digest)
}
