package main

import (
	"bufio"
	"cmp"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	minChangeSeconds = 2.0
	minChangeRatio   = 0.2
	minGainSeconds   = 30.0
)

var testName = regexp.MustCompile(`^Test([A-Z0-9_][A-Za-z0-9_]*)?$`)

type weights struct {
	header []string
	values map[string]float64
}

type change struct {
	name     string
	old, new float64
}

type refresh struct {
	runs           []string
	added, removed []change
	changed        []change
	before, after  float64
	shards         int
}

func main() {
	weightsPath := flag.String("weights", "test/e2e/shard-weights.txt", "weights file to read and, when a refresh is justified, rewrite")
	testsDir := flag.String("tests", "test/e2e", "directory with the e2e test sources")
	shards := flag.Int("shards", 0, "number of e2e shards")
	reportPath := flag.String("report", "", "file to write the PR description to, only when a refresh is justified")
	repo := flag.String("repo", "", "owner/name used to link the runs in the report")
	flag.Parse()

	if *shards < 1 || *reportPath == "" || flag.NArg() == 0 {
		log.Fatal("usage: e2e-shard-weights-refresh -shards N -report FILE [flags] <run-dir>...")
	}

	current, err := readWeights(*weightsPath)
	if err != nil {
		log.Fatal(err)
	}
	tests, err := listTests(*testsDir)
	if err != nil {
		log.Fatal(err)
	}
	var runs []map[string]float64
	for _, dir := range flag.Args() {
		run, err := readRun(dir)
		if err != nil {
			log.Fatal(err)
		}
		runs = append(runs, run)
	}

	medians := median(runs)
	next, r := compute(current.values, tests, medians, *shards)
	for _, dir := range flag.Args() {
		r.runs = append(r.runs, filepath.Base(dir))
	}

	fmt.Printf("slowest of %d shards: %.0fs now, %.0fs refreshed; %d added, %d removed, %d changed\n",
		r.shards, r.before, r.after, len(r.added), len(r.removed), len(r.changed))
	if !r.justified() {
		fmt.Println("refresh not justified; nothing to do")
		return
	}

	if err := writeWeights(*weightsPath, weights{header: current.header, values: next}); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*reportPath, []byte(r.report(*repo)), 0o644); err != nil {
		log.Fatal(err)
	}
}

func readWeights(path string) (weights, error) {
	f, err := os.Open(path)
	if err != nil {
		return weights{}, err
	}
	defer f.Close()

	w := weights{values: map[string]float64{}}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			if len(w.values) == 0 {
				w.header = append(w.header, sc.Text())
			}
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return weights{}, fmt.Errorf("%s: malformed line %q", path, line)
		}
		v, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return weights{}, fmt.Errorf("%s: %w", path, err)
		}
		w.values[fields[0]] = v
	}
	return w, sc.Err()
}

func writeWeights(path string, w weights) error {
	var b strings.Builder
	for _, h := range w.header {
		b.WriteString(h + "\n")
	}
	for _, name := range slices.Sorted(maps.Keys(w.values)) {
		fmt.Fprintf(&b, "%s %.3f\n", name, w.values[name])
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func listTests(dir string) (map[string]bool, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, err
	}
	tests := map[string]bool{}
	fset := token.NewFileSet()
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && isTopLevelTest(fn) {
				tests[fn.Name.Name] = true
			}
		}
	}
	if len(tests) == 0 {
		return nil, fmt.Errorf("no tests found in %s", dir)
	}
	return tests, nil
}

func isTopLevelTest(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || !testName.MatchString(fn.Name.Name) || len(fn.Type.Params.List) != 1 {
		return false
	}
	star, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "testing" && sel.Sel.Name == "T"
}

type junit struct {
	Cases []struct {
		Name string  `xml:"name,attr"`
		Time float64 `xml:"time,attr"`
	} `xml:"testsuite>testcase"`
}

func readRun(dir string) (map[string]float64, error) {
	run := map[string]float64{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".xml" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var report junit
		if err := xml.Unmarshal(data, &report); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, c := range report.Cases {
			if testName.MatchString(c.Name) {
				run[c.Name] = c.Time
			}
		}
		return nil
	})
	if err == nil && len(run) == 0 {
		err = errors.New(dir + ": no test timings found")
	}
	return run, err
}

func median(runs []map[string]float64) map[string]float64 {
	samples := map[string][]float64{}
	for _, run := range runs {
		for name, secs := range run {
			samples[name] = append(samples[name], secs)
		}
	}
	medians := map[string]float64{}
	for name, s := range samples {
		slices.Sort(s)
		mid := len(s) / 2
		if len(s)%2 == 1 {
			medians[name] = round3(s[mid])
		} else {
			medians[name] = round3((s[mid-1] + s[mid]) / 2)
		}
	}
	return medians
}

func compute(current map[string]float64, tests map[string]bool, medians map[string]float64, shards int) (map[string]float64, refresh) {
	r := refresh{shards: shards}
	next := map[string]float64{}
	for _, name := range slices.Sorted(maps.Keys(current)) {
		old := current[name]
		m, measured := medians[name]
		switch {
		case !tests[name]:
			r.removed = append(r.removed, change{name: name, old: old})
		case measured && significant(old, m):
			r.changed = append(r.changed, change{name: name, old: old, new: m})
			next[name] = m
		default:
			next[name] = old
		}
	}
	for _, name := range slices.Sorted(maps.Keys(tests)) {
		if _, known := current[name]; known {
			continue
		}
		if m, measured := medians[name]; measured {
			r.added = append(r.added, change{name: name, new: m})
			next[name] = m
		}
	}
	r.before = slowestShard(current, tests, medians, shards)
	r.after = slowestShard(next, tests, medians, shards)
	return next, r
}

func round3(v float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 3, 64), 64)
	return r
}

func significant(old, m float64) bool {
	d := math.Abs(m - old)
	return d >= minChangeSeconds && d >= minChangeRatio*old
}

type assignment struct {
	name   string
	weight float64
}

// pack assigns tests to shards the same way hack/e2e-shard.sh does.
func pack(w map[string]float64, tests map[string]bool, shards int) [][]assignment {
	mean := 1.0
	if len(w) > 0 {
		var total float64
		for _, v := range w {
			total += v
		}
		mean = total / float64(len(w))
	}

	var items []assignment
	for name := range tests {
		v, ok := w[name]
		if !ok {
			v = mean
		}
		items = append(items, assignment{name: name, weight: round3(v)})
	}
	slices.SortFunc(items, func(a, b assignment) int {
		return cmp.Or(cmp.Compare(b.weight, a.weight), strings.Compare(a.name, b.name))
	})

	packed := make([][]assignment, shards)
	load := make([]float64, shards)
	for _, it := range items {
		best := 0
		for i := 1; i < shards; i++ {
			if load[i] < load[best] {
				best = i
			}
		}
		load[best] += it.weight
		packed[best] = append(packed[best], it)
	}
	return packed
}

func slowestShard(w map[string]float64, tests map[string]bool, medians map[string]float64, shards int) float64 {
	var slowest float64
	for _, shard := range pack(w, tests, shards) {
		var total float64
		for _, it := range shard {
			if m, ok := medians[it.name]; ok {
				total += m
			} else {
				total += it.weight
			}
		}
		slowest = max(slowest, total)
	}
	return slowest
}

func (r refresh) justified() bool {
	return len(r.added) > 0 || len(r.removed) > 0 || r.before-r.after >= minGainSeconds
}

func (r refresh) report(repo string) string {
	var b strings.Builder
	b.WriteString("Refreshes `test/e2e/shard-weights.txt` from the median test durations of these successful master builds:")
	for _, id := range r.runs {
		if repo != "" {
			fmt.Fprintf(&b, " [%s](https://github.com/%s/actions/runs/%s)", id, repo, id)
		} else {
			fmt.Fprintf(&b, " %s", id)
		}
	}
	fmt.Fprintf(&b, "\n\nA duration changes only when the median differs from it by at least %.0fs and %.0f%%.\n\n", minChangeSeconds, minChangeRatio*100)
	fmt.Fprintf(&b, "Estimated slowest of %d shards: %.0fs before, %.0fs after.\n", r.shards, r.before, r.after)

	section := func(title, head string, rows []change, row func(change) string) {
		if len(rows) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n### %s (%d)\n\n%s\n", title, len(rows), head)
		for _, c := range rows {
			b.WriteString(row(c) + "\n")
		}
	}
	section("Added tests", "| Test | Seconds |\n|---|---|", r.added, func(c change) string {
		return fmt.Sprintf("| %s | %.3f |", c.name, c.new)
	})
	section("Removed tests", "| Test | Seconds |\n|---|---|", r.removed, func(c change) string {
		return fmt.Sprintf("| %s | %.3f |", c.name, c.old)
	})
	section("Changed durations", "| Test | Before | After |\n|---|---|---|", r.changed, func(c change) string {
		return fmt.Sprintf("| %s | %.3f | %.3f |", c.name, c.old, c.new)
	})

	b.WriteString("\nThis PR is opened by the `Refresh e2e shard weights` workflow. Its next run overwrites `test/e2e/shard-weights.txt` on this branch, so edit that file elsewhere.\n")
	return b.String()
}
