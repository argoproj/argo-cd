package syncid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	const goroutines = 10
	const idsPerGoroutine = 50
	idsCh := make(chan string, goroutines*idsPerGoroutine)
	errCh := make(chan error, goroutines*idsPerGoroutine)

	// Reset globalCount for deterministic test (not strictly necessary, but can help in CI)
	globalCount.Store(0)

	// Run goroutines in parallel to test for race conditions
	for g := 0; g < goroutines; g++ {
		go func() {
			for i := 0; i < idsPerGoroutine; i++ {
				id, err := Generate()
				if err != nil {
					errCh <- err
					continue
				}
				idsCh <- id
			}
		}()
	}

	ids := make(map[string]any)
	for i := 0; i < goroutines*idsPerGoroutine; i++ {
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case id := <-idsCh:
			assert.Regexp(t, `^\d{5}-[a-zA-Z0-9]{5}$`, id, "ID should match the expected format")
			_, exists := ids[id]
			assert.False(t, exists, "ID should be unique")
			ids[id] = id
		}
	}
}
