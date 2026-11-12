package rbac

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

// cachedPatterns returns the patterns that are in the cache.
func cachedPatterns(c *compiledRegexCache) []string {
	patterns := []string{}
	c.entries.Range(func(pattern, _ any) bool {
		patterns = append(patterns, pattern.(string))
		return true
	})
	return patterns
}

func TestRegexMatchFunc(t *testing.T) {
	ok, err := regexMatchFunc("arg1")
	require.Error(t, err)
	assert.False(t, ok.(bool))

	ok, err = regexMatchFunc("arg1", "arg2", "arg3")
	require.Error(t, err)
	assert.False(t, ok.(bool))

	ok, err = regexMatchFunc(time.Now(), "arg2")
	require.Error(t, err)
	assert.False(t, ok.(bool))

	ok, err = regexMatchFunc("arg1", time.Now())
	require.Error(t, err)
	assert.False(t, ok.(bool))

	ok, err = regexMatchFunc("arg/123", "arg/[0-9]+")
	require.NoError(t, err)
	assert.True(t, ok.(bool))

	ok, err = regexMatchFunc("arg/abc", "arg/[0-9]+")
	require.NoError(t, err)
	assert.False(t, ok.(bool))

	// Like regexp.MatchString, the pattern is not anchored.
	ok, err = regexMatchFunc("applicationsets", "applications")
	require.NoError(t, err)
	assert.True(t, ok.(bool))

	ok, err = regexMatchFunc("applicationsets", "^applications$")
	require.NoError(t, err)
	assert.False(t, ok.(bool))

	ok, err = regexMatchFunc("arg/123", "arg/[0-9")
	require.Error(t, err)
	assert.False(t, ok.(bool))
}

func TestCompiledRegexCache_CachesCompiledPattern(t *testing.T) {
	c := newCompiledRegexCache(10)

	first, err := c.getOrCompile("team-[0-9]+/.*")
	require.NoError(t, err)
	assert.True(t, first.MatchString("team-1/app"))
	assert.Equal(t, []string{"team-[0-9]+/.*"}, cachedPatterns(c))

	second, err := c.getOrCompile("team-[0-9]+/.*")
	require.NoError(t, err)
	assert.Same(t, first, second, "the second call should return the cached pattern rather than compile again")
	assert.Equal(t, []string{"team-[0-9]+/.*"}, cachedPatterns(c))
}

func TestCompiledRegexCache_DoesNotCacheInvalidPattern(t *testing.T) {
	c := newCompiledRegexCache(10)

	_, err := c.getOrCompile("team-[0-9")
	require.Error(t, err)
	assert.Empty(t, cachedPatterns(c))
}

func TestCompiledRegexCache_EmptiesWhenFull(t *testing.T) {
	const maxEntries = 3
	c := newCompiledRegexCache(maxEntries)

	for i := range maxEntries {
		_, err := c.getOrCompile(fmt.Sprintf("pattern-%d", i))
		require.NoError(t, err)
	}
	assert.Len(t, cachedPatterns(c), maxEntries)

	// A hit on a full cache leaves it as it is.
	_, err := c.getOrCompile("pattern-0")
	require.NoError(t, err)
	assert.Len(t, cachedPatterns(c), maxEntries)

	// A miss on a full cache empties it and adds the new pattern.
	compiled, err := c.getOrCompile("pattern-new")
	require.NoError(t, err)
	assert.True(t, compiled.MatchString("pattern-new"))
	assert.Equal(t, []string{"pattern-new"}, cachedPatterns(c))

	// The patterns that were dropped still work, and are cached again.
	compiled, err = c.getOrCompile("pattern-0")
	require.NoError(t, err)
	assert.True(t, compiled.MatchString("pattern-0"))
	assert.ElementsMatch(t, []string{"pattern-new", "pattern-0"}, cachedPatterns(c))
}

func TestCompiledRegexCache_ConcurrentAccess(t *testing.T) {
	// A cache smaller than the pattern set makes hits, misses and resets all happen concurrently.
	const maxEntries = 5
	c := newCompiledRegexCache(maxEntries)

	var wg sync.WaitGroup
	for g := range 16 {
		wg.Go(func() {
			for i := range 200 {
				n := (g + i) % 10
				compiled, err := c.getOrCompile(fmt.Sprintf("^team-%d/.*$", n))
				if err != nil {
					// require must not be used outside the test goroutine.
					t.Errorf("failed to compile pattern: %v", err)
					return
				}
				assert.True(t, compiled.MatchString(fmt.Sprintf("team-%d/app", n)))
				assert.False(t, compiled.MatchString(fmt.Sprintf("team-%d/app", n+1)))
			}
		})
	}
	wg.Wait()

	cached := cachedPatterns(c)
	assert.LessOrEqual(t, len(cached), maxEntries)
	assert.Len(t, cached, c.size)
}

// TestRegexMatchMode_InvalidPattern checks that a policy line with a pattern that does not compile fails the
// enforcement of every request that evaluates the line, even when another line would allow the request.
func TestRegexMatchMode_InvalidPattern(t *testing.T) {
	cm := fakeConfigMap()
	cm.Data[ConfigMapMatchModeKey] = RegexMatchMode
	enf := NewEnforcer(fake.NewClientset(), fakeNamespace, fakeConfigMapName, nil)
	require.NoError(t, enf.syncUpdate(cm, noOpUpdate))
	require.NoError(t, enf.SetUserPolicy(`
p, alice, clusters, get, "https://github.com/argo[a-z", allow
p, alice, clusters, get, "https://github.com/argoproj/.*", allow
p, bob, clusters, get, "https://github.com/argoproj/.*", allow
`))

	assert.False(t, enf.Enforce("alice", "clusters", "get", "https://github.com/argoproj/argo-cd.git"))
	assert.True(t, enf.Enforce("bob", "clusters", "get", "https://github.com/argoproj/argo-cd.git"))
}

// TestRegexMatchMode_PolicyUpdate checks that a pattern cached for one policy does not leak into the decisions made
// for the policy that replaces it.
func TestRegexMatchMode_PolicyUpdate(t *testing.T) {
	cm := fakeConfigMap()
	cm.Data[ConfigMapMatchModeKey] = RegexMatchMode
	enf := NewEnforcer(fake.NewClientset(), fakeNamespace, fakeConfigMapName, nil)
	require.NoError(t, enf.syncUpdate(cm, noOpUpdate))

	require.NoError(t, enf.SetUserPolicy(`p, alice, applications, get, "team-a/.*", allow`))
	assert.True(t, enf.Enforce("alice", "applications", "get", "team-a/app"))
	assert.False(t, enf.Enforce("alice", "applications", "get", "team-b/app"))

	require.NoError(t, enf.SetUserPolicy(`p, alice, applications, get, "team-b/.*", allow`))
	assert.False(t, enf.Enforce("alice", "applications", "get", "team-a/app"))
	assert.True(t, enf.Enforce("alice", "applications", "get", "team-b/app"))
}
