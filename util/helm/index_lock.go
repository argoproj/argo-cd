package helm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var errIndexLockBusy = errors.New("helm index cache record is busy")

// Stable stripes coordinate reclamation of the per-record lock files. Normal
// operations share the stripe and lock only their own record, so a slow origin
// cannot block a fresh record that happens to hash to the same stripe.
func indexRecordLockPath(record string) string {
	digest := sha256.Sum256([]byte(filepath.Base(record)))
	return filepath.Join(filepath.Dir(record), fmt.Sprintf(".helm-index-lock-%02x", digest[0]))
}

// The state binds disk eligibility to one record checksum. An incomplete or invalidated
// state cannot authorize reuse, even after another store or process starts.
func readIndexRecordState(record string) (string, error) {
	file, err := os.OpenFile(record+".state", os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var state [65]byte
	if _, err := file.ReadAt(state[:], 1); err != nil {
		return "", err
	}
	if state[0] != '1' {
		return "", errors.New("helm index disk record is invalidated")
	}
	if _, err := hex.DecodeString(string(state[1:])); err != nil {
		return "", err
	}
	return string(state[1:]), nil
}

// The caller holds the exclusive record lock. Invalidation uses the existing
// state inode so it remains possible when the parent directory forbids unlink.
// Sync invalidation before replacing the JSON record. Publishing the matching
// checksum is the final commit: losing it on a crash leaves durable invalidation,
// and retaining it without the replacement JSON leaves a checksum mismatch.
func writeIndexRecordState(record, digest string) error {
	state := []byte("!")
	if digest != "" {
		if len(digest) != 64 {
			return errors.New("invalid helm index record checksum")
		}
		state = []byte("1" + digest)
	}
	file, err := os.OpenFile(record+".state", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.WriteAt(state, 1); err != nil {
		return err
	}
	if digest == "" {
		return file.Sync()
	}
	return nil
}

// Each operation holds its stripe while opening and using the record lock.
// Removing a record lock requires exclusive ownership of the stripe: no reader
// can still hold or open its previous inode. Stable stripe files remain in place.
func acquireIndexLock(ctx context.Context, record string, shared, wait bool) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Context-free maintenance calls also have a bounded wait.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	stripe := indexRecordLockPath(record)
	unlockStripe, err := acquireIndexFileLock(ctx, stripe, wait, wait)
	if err != nil {
		return nil, err
	}
	lockFile := record + ".lock"
	cleanup := func() {
		if !wait {
			_ = os.Remove(lockFile) // The stripe is still exclusively locked.
			unlockStripe()
			return
		}
		unlockStripe()
		// Reclaim without delaying unrelated operations. A sweep can remove
		// a lock left behind when another reader still shares the stripe.
		if unlock, err := acquireIndexFileLock(context.Background(), stripe, false, false); err == nil {
			_ = os.Remove(lockFile)
			unlock()
		}
	}
	unlockRecord, err := acquireIndexFileLock(ctx, lockFile, shared, wait)
	if err != nil {
		cleanup()
		return nil, err
	}
	return func() {
		unlockRecord()
		cleanup()
	}, nil
}

func acquireIndexFileLock(ctx context.Context, path string, shared, wait bool) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create helm index cache directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open helm index cache lock: %w", err)
	}
	for {
		locked, err := tryIndexLock(file, shared)
		if err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("lock helm index cache record: %w", err)
		}
		if locked {
			release := func() {
				_ = releaseIndexLock(file)
				_ = file.Close()
			}
			if err := ctx.Err(); err != nil {
				release()
				return nil, err
			}
			return release, nil
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
