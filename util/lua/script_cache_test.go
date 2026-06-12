package lua

import (
	"fmt"
	"os"
	"testing"

	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Rollout is a representative built-in health script: ~5 KB of Lua with helper
	// functions and multiple code paths, similar in complexity to hot-path checks.
	scriptCacheBenchScriptPath = "../../resource_customizations/argoproj.io/Rollout/health.lua"
	scriptCacheBenchObjPath    = "../../resource_customizations/argoproj.io/Rollout/testdata/canary/healthy_executedAllSteps.yaml"
)

const cacheTestScriptA = `local hs = {}
hs.status = "Healthy"
hs.message = "a"
return hs
`

func compileProto(t *testing.T, script string) *lua.FunctionProto {
	t.Helper()
	l := lua.NewState()
	defer l.Close()
	fn, err := l.LoadString(script)
	require.NoError(t, err)
	return fn.Proto
}

func TestCompiledScriptCache_ContentAddressed(t *testing.T) {
	c := newCompiledScriptCache()

	_, ok := c.get(cacheTestScriptA)
	assert.False(t, ok, "expected miss for never-seen script")

	proto := compileProto(t, cacheTestScriptA)
	c.add(cacheTestScriptA, proto)

	got, ok := c.get(cacheTestScriptA)
	require.True(t, ok, "expected hit after add")
	assert.Same(t, proto, got, "identical script content must return the same compiled proto")

	// Adding the same key again is a no-op (keeps the original proto).
	c.add(cacheTestScriptA, compileProto(t, cacheTestScriptA))
	got2, _ := c.get(cacheTestScriptA)
	assert.Same(t, proto, got2)

	// A different script body is a distinct key - no invalidation required.
	_, ok = c.get("return 2")
	assert.False(t, ok)
}

func TestCompiledScriptCache_BoundedEviction(t *testing.T) {
	c := newCompiledScriptCache()
	scripts := make([]string, compiledScriptCacheSize+1)
	for i := range scripts {
		scripts[i] = fmt.Sprintf("return %d", i)
		c.add(scripts[i], compileProto(t, scripts[i]))
	}

	assert.Equal(t, compiledScriptCacheSize, c.cache.Len(), "cache must not exceed max size")
	_, ok := c.get(scripts[0])
	assert.False(t, ok, "least-recently-used entry should have been evicted")
	_, ok = c.get(scripts[len(scripts)-1])
	assert.True(t, ok, "newest entry should be present")
}

func TestCompiledScriptCache_OnOffParity(t *testing.T) {
	testObj := StrToUnstructured(objJSON)

	orig := scriptCacheEnabled
	t.Cleanup(func() { scriptCacheEnabled = orig })

	vm := VM{UseOpenLibs: true}

	scriptCacheEnabled = false
	off, err := vm.ExecuteHealthLua(testObj, cacheTestScriptA)
	require.NoError(t, err)

	scriptCacheEnabled = true
	on, err := vm.ExecuteHealthLua(testObj, cacheTestScriptA)
	require.NoError(t, err)

	assert.Equal(t, off, on, "cache must not change health output")
}

func loadScriptCacheBenchFixtures(tb testing.TB) (script string, obj *unstructured.Unstructured) {
	tb.Helper()
	scriptBytes, err := os.ReadFile(scriptCacheBenchScriptPath)
	require.NoError(tb, err)
	yamlBytes, err := os.ReadFile(scriptCacheBenchObjPath)
	require.NoError(tb, err)
	objMap := make(map[string]any)
	require.NoError(tb, yaml.Unmarshal(yamlBytes, &objMap))
	return string(scriptBytes), &unstructured.Unstructured{Object: objMap}
}

func BenchmarkExecuteHealthLuaScriptCache(b *testing.B) {
	script, obj := loadScriptCacheBenchFixtures(b)
	vm := VM{}

	for _, mode := range []struct {
		name    string
		enabled bool
	}{
		{"cacheOff", false},
		{"cacheOn", true},
	} {
		b.Run(mode.name, func(b *testing.B) {
			origEnabled := scriptCacheEnabled
			origCache := compiledScripts
			scriptCacheEnabled = mode.enabled
			if mode.enabled {
				compiledScripts = newCompiledScriptCache()
				if _, err := vm.ExecuteHealthLua(obj, script); err != nil {
					b.Fatal(err)
				}
			}
			b.Cleanup(func() {
				scriptCacheEnabled = origEnabled
				compiledScripts = origCache
			})

			b.ReportAllocs()
			for b.Loop() {
				if _, err := vm.ExecuteHealthLua(obj, script); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
