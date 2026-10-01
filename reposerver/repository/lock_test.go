package repository

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// execute given action and return false if action have not completed within 1 second
func lockQuickly(action func() (io.Closer, error)) (io.Closer, bool) {
	done := make(chan io.Closer)
	go func() {
		closer, _ := action()
		done <- closer
	}()
	select {
	case <-time.After(1 * time.Second):
		return nil, false
	case closer := <-done:
		return closer, true
	}
}

func numberOfInits(initializedTimes *int) func(_ bool) (io.Closer, error) {
	return func(_ bool) (io.Closer, error) {
		*initializedTimes++
		return utilio.NopCloser, nil
	}
}

func TestLock_SameRevision(t *testing.T) {
	t.Parallel()
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)
	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	closer2, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	assert.Equal(t, 1, initializedTimes)

	utilio.Close(closer1)

	utilio.Close(closer2)
}

func TestLock_DifferentRevisions(t *testing.T) {
	t.Parallel()
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)

	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "2", true, init)
	})

	if !assert.False(t, done) {
		return
	}

	utilio.Close(closer1)

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "2", true, init)
	})

	if !assert.True(t, done) {
		return
	}
}

func TestLock_NoConcurrentWithSameRevision(t *testing.T) {
	t.Parallel()
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)

	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", false, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", false, init)
	})

	if !assert.False(t, done) {
		return
	}

	utilio.Close(closer1)
}

func TestLock_FailedInitialization(t *testing.T) {
	t.Parallel()
	lock := NewRepositoryLock()

	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, func(_ bool) (io.Closer, error) {
			return utilio.NopCloser, errors.New("failed")
		})
	})

	if !assert.True(t, done) {
		return
	}

	assert.Nil(t, closer1)

	closer2, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, func(_ bool) (io.Closer, error) {
			return utilio.NopCloser, nil
		})
	})

	if !assert.True(t, done) {
		return
	}

	utilio.Close(closer2)
}

func TestLock_SameRevisionFirstNotConcurrent(t *testing.T) {
	t.Parallel()
	lock := NewRepositoryLock()
	initializedTimes := 0
	init := numberOfInits(&initializedTimes)
	closer1, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", false, init)
	})

	if !assert.True(t, done) {
		return
	}

	_, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	if !assert.False(t, done) {
		return
	}

	assert.Equal(t, 1, initializedTimes)

	utilio.Close(closer1)
}

func TestLock_CleanForNonConcurrent(t *testing.T) {
	t.Parallel()
	lock := NewRepositoryLock()
	initClean := false
	init := func(clean bool) (io.Closer, error) {
		initClean = clean
		return utilio.NopCloser, nil
	}
	closer, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	assert.True(t, done)
	// first time always clean because we cannot be sure about the state of repository
	assert.True(t, initClean)
	utilio.Close(closer)

	closer, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})

	assert.True(t, done)
	assert.False(t, initClean)
	utilio.Close(closer)
}

func TestLock_CleanOnRevisionChange(t *testing.T) {
	t.Parallel()
	lock := NewRepositoryLock()
	var cleanValues []bool
	init := func(clean bool) (io.Closer, error) {
		cleanValues = append(cleanValues, clean)
		return utilio.NopCloser, nil
	}

	// First op: revision "1", concurrent allowed.
	closer, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "1", true, init)
	})
	assert.True(t, done)
	// First init is always clean (unknown initial state).
	assert.True(t, cleanValues[0])
	utilio.Close(closer)

	// Second op: revision "2" (different!), concurrent allowed.
	closer, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "2", true, init)
	})
	assert.True(t, done)
	// Revision changed, but a full clean is skipped: it is too expensive for
	// large repositories (https://github.com/argoproj/argo-cd/issues/29856).
	// Instead, the Helm dependency build markers are removed so that
	// `helm dependency build` is re-run for the new revision.
	assert.False(t, cleanValues[1])
	utilio.Close(closer)

	// Third op: same revision "2" again, concurrent allowed - no clean needed.
	closer, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock("myRepo", "2", true, init)
	})
	assert.True(t, done)
	assert.False(t, cleanValues[2])
	utilio.Close(closer)
}

func TestLock_RemoveHelmDepMarkersOnRevisionChange(t *testing.T) {
	t.Parallel()
	lock := NewRepositoryLock()
	repoRoot := t.TempDir()
	init := func(_ bool) (io.Closer, error) {
		return utilio.NopCloser, nil
	}

	// Process revision "rev1", leaving behind a Helm dependency build marker
	// in the repository working copy.
	closer, done := lockQuickly(func() (io.Closer, error) {
		return lock.Lock(repoRoot, "rev1", true, init)
	})
	assert.True(t, done)
	markerFile := filepath.Join(repoRoot, "my-chart", helmDepUpMarkerFile)
	assert.NoError(t, os.MkdirAll(filepath.Dir(markerFile), 0o755))
	assert.NoError(t, os.WriteFile(markerFile, []byte("marker"), 0o644))
	helmDepMarkers.add(repoRoot, markerFile)
	utilio.Close(closer)

	// Process the same revision again: the marker must be preserved so that
	// `helm dependency build` is not run again unnecessarily.
	closer, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(repoRoot, "rev1", true, init)
	})
	assert.True(t, done)
	utilio.Close(closer)
	_, err := os.Stat(markerFile)
	require.NoError(t, err)

	// Process another revision: the marker left by the previous revision must
	// be removed so that `helm dependency build` is re-run.
	closer, done = lockQuickly(func() (io.Closer, error) {
		return lock.Lock(repoRoot, "rev2", true, init)
	})
	assert.True(t, done)
	utilio.Close(closer)
	_, err = os.Stat(markerFile)
	assert.True(t, os.IsNotExist(err))
}
