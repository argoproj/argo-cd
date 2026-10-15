package main

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func set(names ...string) map[string]bool {
	s := map[string]bool{}
	for _, n := range names {
		s[n] = true
	}
	return s
}

func TestWeightsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weights.txt")
	writeFile(t, path, "# header one\n# header two\nTestB 2.500\nTestA 1.000\n\n")

	w, err := readWeights(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"# header one", "# header two"}, w.header)
	assert.Equal(t, map[string]float64{"TestA": 1, "TestB": 2.5}, w.values)

	require.NoError(t, writeWeights(path, w))
	out, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "# header one\n# header two\nTestA 1.000\nTestB 2.500\n", string(out))
}

func TestReadWeightsRejectsMalformedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weights.txt")
	writeFile(t, path, "TestA 1.0 extra\n")
	_, err := readWeights(path)
	require.ErrorContains(t, err, "malformed")
}

func TestListTests(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a_test.go"), `package e2e

import "testing"

func TestOne(t *testing.T) {}
func TestTwo(_ *testing.T) {}
func TestMain(m *testing.M) {}
func TestWithArgs(t *testing.T, n int) {}
func BenchmarkOne(b *testing.B) {}
func testHelper(t *testing.T) {}
func Testlower(t *testing.T) {}

type s struct{}

func (s) TestMethod(t *testing.T) {}
`)
	writeFile(t, filepath.Join(dir, "helpers.go"), "package e2e\n\nfunc TestNotInTestFile(t *testing.T) {}\n")

	tests, err := listTests(dir)
	require.NoError(t, err)
	assert.Equal(t, set("TestOne", "TestTwo", "Testlower"), tests)
}

func TestListTestsFailsWithoutTests(t *testing.T) {
	_, err := listTests(t.TempDir())
	require.ErrorContains(t, err, "no tests found")
}

func TestReadRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "e2e-test-timings-shard1", "junit.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="github.com/argoproj/argo-cd/v3/test/e2e">
    <testcase classname="github.com/argoproj/argo-cd/v3/test/e2e" name="TestA/sub" time="1.000"></testcase>
    <testcase classname="github.com/argoproj/argo-cd/v3/test/e2e" name="TestA" time="2.000"></testcase>
    <testcase classname="github.com/argoproj/argo-cd/v3/test/e2e" name="TestFlaky" time="9.000"><failure>boom</failure></testcase>
    <testcase classname="github.com/argoproj/argo-cd/v3/test/e2e" name="TestFlaky" time="3.000"></testcase>
  </testsuite>
</testsuites>`)
	writeFile(t, filepath.Join(dir, "e2e-test-timings-shard2", "junit.xml"), `<testsuites><testsuite><testcase name="TestB" time="4.5"></testcase></testsuite></testsuites>`)
	writeFile(t, filepath.Join(dir, "notes.txt"), "ignored")

	run, err := readRun(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"TestA": 2, "TestFlaky": 3, "TestB": 4.5}, run)
}

func TestReadRunFailsWithoutTimings(t *testing.T) {
	_, err := readRun(t.TempDir())
	require.ErrorContains(t, err, "no test timings found")
}

func TestMedian(t *testing.T) {
	runs := []map[string]float64{
		{"TestOdd": 5, "TestEven": 1, "TestOnce": 7},
		{"TestOdd": 1, "TestEven": 4},
		{"TestOdd": 3},
	}
	assert.Equal(t, map[string]float64{"TestOdd": 3, "TestEven": 2.5, "TestOnce": 7}, median(runs))
}

func TestSignificant(t *testing.T) {
	tests := []struct {
		old, median float64
		want        bool
	}{
		{10, 11.5, false},
		{10, 13, true},
		{10, 7, true},
		{1, 2.9, false},
		{100, 119, false},
		{100, 120, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, significant(tt.old, tt.median), "old %v median %v", tt.old, tt.median)
	}
}

func TestCompute(t *testing.T) {
	current := map[string]float64{"TestKept": 10, "TestNoise": 10, "TestSlower": 10, "TestGone": 5, "TestUnmeasured": 4}
	tests := set("TestKept", "TestNoise", "TestSlower", "TestUnmeasured", "TestNew", "TestNewUnmeasured")
	medians := map[string]float64{"TestKept": 10, "TestNoise": 11.5, "TestSlower": 20, "TestGone": 5, "TestNew": 3}

	next, r := compute(current, tests, medians, 2)

	assert.Equal(t, map[string]float64{"TestKept": 10, "TestNoise": 10, "TestSlower": 20, "TestUnmeasured": 4, "TestNew": 3}, next)
	assert.Equal(t, []change{{name: "TestNew", new: 3}}, r.added)
	assert.Equal(t, []change{{name: "TestGone", old: 5}}, r.removed)
	assert.Equal(t, []change{{name: "TestSlower", old: 10, new: 20}}, r.changed)
	assert.Equal(t, 2, r.shards)
}

func TestSlowestShard(t *testing.T) {
	tests := set("TestA", "TestB", "TestC", "TestD")
	w := map[string]float64{"TestA": 10, "TestB": 6, "TestC": 5, "TestD": 1}

	assert.Equal(t, [][]assignment{
		{{"TestA", 10}, {"TestD", 1}},
		{{"TestB", 6}, {"TestC", 5}},
	}, pack(w, tests, 2))

	assert.InDelta(t, 11.0, slowestShard(w, tests, map[string]float64{}, 2), 1e-9)
	assert.InDelta(t, 31.0, slowestShard(w, tests, map[string]float64{"TestA": 30}, 2), 1e-9)
	assert.InDelta(t, 21.0, slowestShard(w, tests, map[string]float64{"TestD": 11}, 2), 1e-9)
}

func TestPackGivesUnknownTestsTheMeanWeight(t *testing.T) {
	packed := pack(map[string]float64{"TestA": 2, "TestB": 4}, set("TestA", "TestB", "TestNew"), 1)
	assert.Equal(t, [][]assignment{{{"TestB", 4}, {"TestNew", 3}, {"TestA", 2}}}, packed)
}

func TestPackMatchesShardScript(t *testing.T) {
	weightsPath := filepath.Join("..", "..", "test", "e2e", "shard-weights.txt")
	w, err := readWeights(weightsPath)
	require.NoError(t, err)

	names := slices.Sorted(maps.Keys(w.values))
	tests := set(names[3:]...)
	tests["TestNotInWeightsA"] = true
	tests["TestNotInWeightsB"] = true

	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, "go"), "#!/bin/sh\nprintf '%s\\n' "+strings.Join(slices.Sorted(maps.Keys(tests)), " ")+"\necho 'ok pkg 0.1s'\n")
	require.NoError(t, os.Chmod(filepath.Join(bin, "go"), 0o755))

	for _, shards := range []int{1, 3, 4} {
		for i, shard := range pack(w.values, tests, shards) {
			cmd := exec.CommandContext(t.Context(), filepath.Join("..", "e2e-shard.sh"), "pkg", strconv.Itoa(i+1), strconv.Itoa(shards))
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "LC_ALL=C", "ARGOCD_E2E_SHARD_WEIGHTS="+weightsPath)
			out, err := cmd.Output()
			require.NoError(t, err)

			regex := strings.TrimSpace(string(out))
			got := strings.Split(strings.TrimSuffix(strings.TrimPrefix(regex, "^("), ")$"), "|")
			var want []string
			for _, a := range shard {
				want = append(want, a.name)
			}
			assert.Equal(t, want, got, "shard %d of %d", i+1, shards)
		}
	}
}

func TestJustified(t *testing.T) {
	assert.False(t, refresh{before: 100, after: 71}.justified())
	assert.True(t, refresh{before: 100, after: 70}.justified())
	assert.True(t, refresh{before: 100, after: 120, added: []change{{name: "TestNew"}}}.justified())
	assert.True(t, refresh{before: 100, after: 100, removed: []change{{name: "TestGone"}}}.justified())
}

func TestReport(t *testing.T) {
	r := refresh{
		runs:    []string{"111", "222"},
		added:   []change{{name: "TestNew", new: 3}},
		changed: []change{{name: "TestSlower", old: 10, new: 20}},
		before:  950.4,
		after:   900.6,
		shards:  4,
	}

	out := r.report("argoproj/argo-cd")
	assert.Contains(t, out, "[111](https://github.com/argoproj/argo-cd/actions/runs/111) [222](https://github.com/argoproj/argo-cd/actions/runs/222)")
	assert.Contains(t, out, "Estimated slowest of 4 shards: 950s before, 901s after.")
	assert.Contains(t, out, "### Added tests (1)\n\n| Test | Seconds |\n|---|---|\n| TestNew | 3.000 |\n")
	assert.Contains(t, out, "### Changed durations (1)\n\n| Test | Before | After |\n|---|---|---|\n| TestSlower | 10.000 | 20.000 |\n")
	assert.NotContains(t, out, "Removed tests")

	assert.Contains(t, r.report(""), "builds: 111 222\n")
}
