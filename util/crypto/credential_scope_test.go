package crypto

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCredentialScopeStableAcrossIndependentCaches(t *testing.T) {
	first := newCredentialScopeCache(4, deriveCredentialScope)
	second := newCredentialScopeCache(4, deriveCredentialScope)
	require.NotEqual(t, first.lookupKey, second.lookupKey)
	fields := []string{"reader", "private-password", string([]byte{0xff, 0, 0xfe})}
	scope := first.scope("helm", fields...)
	require.Equal(t, scope, second.scope("helm", fields...), "process-local lookup keys must not change persisted identities")
	require.Len(t, strings.TrimPrefix(scope, "argon2id-v1:"), 64, "persist the full 256-bit result")
	require.NotContains(t, scope, "private-password")
	require.NotEqual(t, scope, first.scope("oci", fields...), "protocol domains must remain isolated")
	require.NotEqual(t, first.scope("helm", "a\x00b", "c"), first.scope("helm", "a", "b\x00c"), "credential field boundaries must be unambiguous")
}

func TestCredentialScopeAcrossProcesses(t *testing.T) {
	if os.Getenv("ARGOCD_TEST_CREDENTIAL_SCOPE_HELPER") == "1" {
		fmt.Print(CredentialScope("process-test", "reader", "fixture-password"))
		os.Exit(0)
	}
	binary, err := os.Executable()
	require.NoError(t, err)
	command := exec.CommandContext(t.Context(), binary, "-test.run=^TestCredentialScopeAcrossProcesses$")
	command.Env = append(os.Environ(), "ARGOCD_TEST_CREDENTIAL_SCOPE_HELPER=1")
	output, err := command.Output()
	require.NoError(t, err)
	require.Equal(t, CredentialScope("process-test", "reader", "fixture-password"), string(output), "restarted processes must address the same persisted cache records")
}

func TestCredentialScopeCacheBoundedAndReusesWarmEntries(t *testing.T) {
	var calls atomic.Int32
	cache := newCredentialScopeCache(2, func(_, _ []byte) []byte {
		calls.Add(1)
		return bytes.Repeat([]byte{1}, 32)
	})
	cache.scope("helm", "a")
	cache.scope("helm", "b")
	cache.scope("helm", "a")
	require.EqualValues(t, 2, calls.Load(), "warm lookups must not repeat the KDF")
	cache.scope("helm", "c")
	cache.scope("helm", "a")
	require.EqualValues(t, 3, calls.Load(), "the recently used entry must survive eviction")
	cache.scope("helm", "b")
	require.EqualValues(t, 4, calls.Load(), "the least recently used entry must be recomputed")
	require.Equal(t, 2, cache.values.Len())
}

func TestCredentialScopeConcurrentMissesDoNotBlockWarmHits(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	cache := newCredentialScopeCache(4, func(_, _ []byte) []byte {
		if calls.Add(1) == 2 {
			close(started)
			<-release
		}
		return bytes.Repeat([]byte{1}, 32)
	})
	warm := cache.scope("helm", "warm")
	const workers = 20
	results := make(chan string, workers)
	for range workers {
		go func() { results <- cache.scope("helm", "cold") }()
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("cold derivation did not start")
	}
	warmResult := make(chan string, 1)
	go func() { warmResult <- cache.scope("helm", "warm") }()
	select {
	case got := <-warmResult:
		require.Equal(t, warm, got)
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("a cold derivation blocked an unrelated warm lookup")
	}
	close(release)
	for range workers {
		select {
		case <-results:
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent credential lookups did not finish")
		}
	}
	require.EqualValues(t, 2, calls.Load(), "concurrent misses must derive a view only once")
}

func BenchmarkCredentialScope(b *testing.B) {
	b.Run("cold", func(b *testing.B) {
		cache := newCredentialScopeCache(1, deriveCredentialScope)
		b.ReportAllocs()
		b.ResetTimer()
		for i := range b.N {
			cache.scope("helm", "reader", strconv.Itoa(i))
		}
	})
	b.Run("warm", func(b *testing.B) {
		cache := newCredentialScopeCache(1, deriveCredentialScope)
		cache.scope("helm", "reader", "credential")
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			cache.scope("helm", "reader", "credential")
		}
	})
}
