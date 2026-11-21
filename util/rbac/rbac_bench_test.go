package rbac

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

// newBenchmarkEnforcer returns an enforcer in the given match mode with a role that has policyLines policy lines, and
// the prefix of the objects that the last of those lines allows.
func newBenchmarkEnforcer(b *testing.B, matchMode string, objSuffix string, policyLines int) (*Enforcer, string) {
	b.Helper()
	cm := fakeConfigMap()
	cm.Data[ConfigMapMatchModeKey] = matchMode
	enf := NewEnforcer(fake.NewClientset(), fakeNamespace, fakeConfigMapName, nil)
	require.NoError(b, enf.syncUpdate(cm, noOpUpdate))

	var policy strings.Builder
	for i := range policyLines {
		fmt.Fprintf(&policy, "p, role:dev, applications, get, team-%d/%s, allow\n", i, objSuffix)
	}
	policy.WriteString("g, alice, role:dev\n")
	require.NoError(b, enf.SetUserPolicy(policy.String()))

	objPrefix := fmt.Sprintf("team-%d/app-", policyLines-1)
	require.True(b, enf.Enforce("alice", "applications", "get", objPrefix+"warmup"))
	return enf, objPrefix
}

// benchmarkEnforce measures an enforcement decision that misses casbin's result cache. Every iteration enforces a
// different object, which is what happens when the API server filters a list of applications for a user.
func benchmarkEnforce(b *testing.B, matchMode string, objSuffix string, policyLines int) {
	b.Helper()
	enf, objPrefix := newBenchmarkEnforcer(b, matchMode, objSuffix, policyLines)

	i := 0
	b.ReportAllocs()
	for b.Loop() {
		if !enf.Enforce("alice", "applications", "get", objPrefix+strconv.Itoa(i)) {
			b.Fatal("expected request to be allowed")
		}
		i++
	}
}

// benchmarkEnforceParallel is benchmarkEnforce with concurrent callers, as when the API server handles many requests
// at once.
func benchmarkEnforceParallel(b *testing.B, matchMode string, objSuffix string, policyLines int) {
	b.Helper()
	enf, objPrefix := newBenchmarkEnforcer(b, matchMode, objSuffix, policyLines)

	var next atomic.Int64
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !enf.Enforce("alice", "applications", "get", objPrefix+strconv.FormatInt(next.Add(1), 10)) {
				b.Error("expected request to be allowed")
				return
			}
		}
	})
}

func BenchmarkEnforce_GlobMatchMode(b *testing.B) {
	for _, lines := range []int{10, 50, 200} {
		b.Run(fmt.Sprintf("policyLines=%d", lines), func(b *testing.B) {
			benchmarkEnforce(b, GlobMatchMode, "*", lines)
		})
	}
}

func BenchmarkEnforce_RegexMatchMode(b *testing.B) {
	for _, lines := range []int{10, 50, 200} {
		b.Run(fmt.Sprintf("policyLines=%d", lines), func(b *testing.B) {
			benchmarkEnforce(b, RegexMatchMode, ".*", lines)
		})
	}
}

func BenchmarkEnforce_GlobMatchMode_Parallel(b *testing.B) {
	benchmarkEnforceParallel(b, GlobMatchMode, "*", 50)
}

func BenchmarkEnforce_RegexMatchMode_Parallel(b *testing.B) {
	benchmarkEnforceParallel(b, RegexMatchMode, ".*", 50)
}
