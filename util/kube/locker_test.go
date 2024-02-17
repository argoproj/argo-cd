package kube

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLock(t *testing.T) {
	cli := fake.NewSimpleClientset()

	t.Run("new a lock", func(t *testing.T) {
		require.NotNil(t, NewLocker(cli, DefaultLockHoldingDuration, context.Background()))
	})
	t.Run("perform lock & unlock process without race", func(t *testing.T) {
		lockKey := "test-lock"
		ns := "test-for-lock"
		l := NewLocker(cli, DefaultLockHoldingDuration, context.Background())
		t.Run("lock", func(t *testing.T) {
			err := l.Lock(ns, lockKey)
			require.NoError(t, err)
		})
		t.Run("unlock", func(t *testing.T) {
			err := l.Unlock(ns, lockKey)
			require.NoError(t, err)
		})
	})
	t.Run("try lock in a race", func(t *testing.T) {
		num := 10
		lockKey := "test-lock1"
		ns := "test-for-lock"
		l := NewLocker(cli, DefaultLockHoldingDuration, context.Background())
		successCount := int32(0)
		wg := sync.WaitGroup{}
		for i := 0; i < num; i++ {
			wg.Add(1)
			go func() {
				result := l.TryLock(ns, lockKey)
				if result.success {
					atomic.AddInt32(&successCount, 1)
				}
				wg.Done()
			}()
		}
		wg.Wait()
		assert.EqualValues(t, 1, successCount)
	})

	t.Run("unlock when resource not exist", func(t *testing.T) {
		lockKey := "test-lock"
		ns := "test-for-lock"
		l := NewLocker(cli, DefaultLockHoldingDuration, context.Background())

		require.NoError(t, l.Lock(ns, lockKey))
		require.NoError(t, cli.CoreV1().ConfigMaps(ns).Delete(context.Background(), lockName(lockKey), metav1.DeleteOptions{}))
		require.NoError(t, l.Unlock(ns, lockKey))
	})

	t.Run("lock when resource not exist", func(t *testing.T) {
		lockKey := "test-lock"
		ns := "test-for-lock"
		l := NewLocker(cli, DefaultLockHoldingDuration, context.Background())

		require.NoError(t, l.Lock(ns, lockKey))
		require.NoError(t, cli.CoreV1().ConfigMaps(ns).Delete(context.Background(), lockName(lockKey), metav1.DeleteOptions{}))
		require.NoError(t, l.Lock(ns, lockKey))
		require.NoError(t, l.Unlock(ns, lockKey))
	})

	t.Run("lock when last locker expired", func(t *testing.T) {
		lockKey := "test-aaa"
		ns := "test-for-lock-timeout"
		l := NewLocker(cli, time.Nanosecond, context.Background())
		require.NoError(t, l.Lock(ns, lockKey))
		time.Sleep(time.Microsecond)
		require.NoError(t, l.Lock(ns, lockKey))
		require.NoError(t, l.Unlock(ns, lockKey))
	})
}
