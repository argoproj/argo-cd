package helm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var errIndexLockBusy = errors.New("helm index cache record is busy")

// Each operation opens its own descriptor. The OS lock coordinates goroutines,
// store instances and processes, and is released even if a process crashes.
// Lock files must remain in place: unlinking one permits two independent locks.
func acquireIndexLock(ctx context.Context, record string, shared, wait bool) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(record), 0o700); err != nil {
		return nil, fmt.Errorf("create helm index cache directory: %w", err)
	}
	file, err := os.OpenFile(record+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open helm index cache lock: %w", err)
	}
	// Context-free maintenance calls also have a bounded wait.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	for {
		locked, err := tryIndexLock(file, shared)
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("lock helm index cache record: %w", err)
		}
		if locked {
			if err := ctx.Err(); err != nil {
				_ = file.Close()
				return nil, err
			}
			return func() { _ = file.Close() }, nil
		}
		if !wait {
			_ = file.Close()
			return nil, errIndexLockBusy
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = file.Close()
			return nil, fmt.Errorf("wait for helm index cache lock: %w", ctx.Err())
		case <-timer.C:
		}
	}
}
