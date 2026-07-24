package webhook

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartWorkerPoolProcessesAfterPanicAndShutsDown(t *testing.T) {
	queue := make(chan any, 3)
	queue <- "first"
	queue <- "panic"
	queue <- "last"
	close(queue)

	var waitGroup sync.WaitGroup
	var lock sync.Mutex
	var processed []string
	StartWorkerPool(&waitGroup, queue, 1, "test-webhook", "test panic", func(payload any) {
		value := payload.(string)
		if value == "panic" {
			panic("boom")
		}
		lock.Lock()
		defer lock.Unlock()
		processed = append(processed, value)
	})

	waitGroup.Wait()
	require.Equal(t, []string{"first", "last"}, processed)
}
