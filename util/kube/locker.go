package kube

import (
	"context"
	goerr "errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

const (
	ArgoLockPrefix             = "argo-lock-"
	DeadlineTimeFormat         = time.RFC3339
	DefaultLockHoldingDuration = 2 * time.Second
	DefaultTryTimes            = 3
)

const (
	ArgoLockExpiredAnnotationKey = "argocd.argoproj.io/lock-expired-at"
)

type LockResult struct {
	Err     error
	TryNext time.Duration
	success bool
}

type Locker struct {
	client          kubernetes.Interface
	ctx             context.Context
	holdingDuration time.Duration
}

var (
	TryImmediately = LockResult{}
	Success        = LockResult{success: true}

	LockFailOutOfTimes = goerr.New("lock fail")
)

func lockName(key string) string {
	return ArgoLockPrefix + key
}

func (l *Locker) Lock(namespace, key string) error {
	for i := 0; i < DefaultTryTimes; i++ {
		res := l.TryLock(namespace, key)
		if res.success {
			return nil
		}
		if res.Err != nil {
			return res.Err
		}
		time.Sleep(res.TryNext)
	}

	return LockFailOutOfTimes
}

func (l *Locker) TryLock(namespace, key string) LockResult {
	resName := lockName(key)
	resource := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resName,
			Namespace: namespace,
			Annotations: map[string]string{
				ArgoLockExpiredAnnotationKey: time.Now().Add(l.holdingDuration).Format(DeadlineTimeFormat),
			},
		},
	}

	_, err := l.client.CoreV1().ConfigMaps(namespace).Create(l.ctx, resource, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			old, err := l.client.CoreV1().ConfigMaps(namespace).Get(l.ctx, resName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					// if not found, try lock immediately
					return TryImmediately
				}
				return LockResult{Err: err}
			}
			d := l.calNextTimeToTry(old.Annotations[ArgoLockExpiredAnnotationKey])
			if d <= 0 {
				_ = l.Unlock(namespace, key)
				return TryImmediately
			}
			return LockResult{TryNext: d}
		} else {
			return LockResult{Err: err}
		}
	}
	return Success
}

// Unlock delete the configmap
func (l *Locker) Unlock(namespace, key string) error {
	resName := lockName(key)
	err := l.client.CoreV1().ConfigMaps(namespace).Delete(l.ctx, resName, metav1.DeleteOptions{})
	if err == nil || errors.IsNotFound(err) {
		return nil
	}
	return err
}

// calNextTimeToTry calculate the time we should wait,if any unexpected error occur, we treat it as expired.
func (l *Locker) calNextTimeToTry(timeStr string) time.Duration {
	deadline, err := time.Parse(DeadlineTimeFormat, timeStr)
	if err != nil {
		return -1
	}
	return time.Until(deadline)
}

func NewLocker(client kubernetes.Interface, duration time.Duration, ctx context.Context) *Locker {
	return &Locker{client: client, ctx: ctx, holdingDuration: duration}
}
