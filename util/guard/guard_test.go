package guard

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

type nop struct{}

func (nop) Errorf(string, ...any) {}

// recorder is a thread-safe logger that captures formatted messages.
type recorder struct {
	mu    sync.Mutex
	calls int
	msgs  []string
}

func (r *recorder) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.msgs = append(r.msgs, fmt.Sprintf(format, args...))
}

func TestRun_Recovers(_ *testing.T) {
	RecoverAndLog(func() { panic("boom") }, nop{}, "msg") // fails if panic escapes
}

func TestRun_AllowsNextCall(t *testing.T) {
	ran := false
	RecoverAndLog(func() { panic("boom") }, nop{}, "msg")
	RecoverAndLog(func() { ran = true }, nop{}, "msg")
	if !ran {
		t.Fatal("expected second callback to run")
	}
}

func TestRun_LogsMessageAndStack(t *testing.T) {
	r := &recorder{}
	RecoverAndLog(func() { panic("boom") }, r, "msg")
	if r.calls != 1 {
		t.Fatalf("expected 1 log call, got %d", r.calls)
	}
	got := strings.Join(r.msgs, "\n")
	if !strings.Contains(got, "msg") {
		t.Errorf("expected log to contain message %q; got %q", "msg", got)
	}
	if !strings.Contains(got, "boom") {
		t.Errorf("expected log to contain panic value %q; got %q", "boom", got)
	}
	// Heuristic check that a stack trace was included.
	if !strings.Contains(got, "guard.go") && !strings.Contains(got, "runtime/panic.go") && !strings.Contains(got, "goroutine") {
		t.Errorf("expected log to contain a stack trace; got %q", got)
	}
}

func TestRun_NilLoggerDoesNotPanic(_ *testing.T) {
	var l Logger // nil
	RecoverAndLog(func() { panic("boom") }, l, "ignored")
}

func TestRun_NoPanicDoesNotLog(t *testing.T) {
	r := &recorder{}
	ran := false
	RecoverAndLog(func() { ran = true }, r, "msg")
	if !ran {
		t.Fatal("expected fn to run")
	}
	if r.calls != 0 {
		t.Fatalf("expected 0 log calls, got %d", r.calls)
	}
}

func TestRun_ConcurrentPanicsLogged(t *testing.T) {
	r := &recorder{}
	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			RecoverAndLog(func() { panic(fmt.Sprintf("boom-%d", i)) }, r, "msg")
		}(i)
	}
	wg.Wait()
	if r.calls != n {
		t.Fatalf("expected %d log calls, got %d", n, r.calls)
	}
}
