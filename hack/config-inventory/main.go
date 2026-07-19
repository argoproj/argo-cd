// Command config-inventory discovers current Argo CD config sources and reports
// registry coverage. It does NOT assign CRD paths.
//
//	go run ./hack/config-inventory -repo-root . -out /tmp/config-inventory.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/argoproj/argo-cd/v3/util/configbus"
)

type Inventory struct {
	CMKeys     []string          `json:"cmKeys"`
	EnvVars    []EnvVar          `json:"envVars"`
	CmdParams  []CmdParamRef     `json:"cmdParams"`
	Flags      []FlagRef         `json:"flags"`
	Uncovered  UncoveredReport   `json:"uncovered,omitempty"`
	Registered []RegisteredEntry `json:"registered,omitempty"`
}

type EnvVar struct {
	Name    string `json:"name"`
	Default string `json:"default,omitempty"`
	Helper  string `json:"helper"`
	Package string `json:"package"`
	File    string `json:"file"`
	// FlagBound is true when this env read supplies the default for a cobra/pflag
	// flag binding (e.g. cmd.Flags().StringVar(..., env.StringFromEnv(...), ...)).
	// Flag-bound env vars are just the transport for argocd-cmd-params-cm keys and
	// are populated on component structs by cobra parsing, not loaded by the
	// provider directly. Env reads that are NOT flag-bound are the truly one-off
	// env vars the provider should own.
	FlagBound bool `json:"flagBound"`
}

// FlagRef is a discovered cobra/pflag binding.
type FlagRef struct {
	Name      string `json:"name"`
	Component string `json:"component"`
	File      string `json:"file"`
	HasEnv    bool   `json:"hasEnv"`
	EnvVar    string `json:"envVar,omitempty"`
	// PureFlag is true when the flag has no env-derived default (no CM/env transport).
	PureFlag bool `json:"pureFlag"`
}

type CmdParamRef struct {
	CMKey  string `json:"cmKey"`
	EnvVar string `json:"envVar"`
	File   string `json:"file"`
}

type UncoveredReport struct {
	CMKeys  []string `json:"cmKeys"`
	EnvVars []string `json:"envVars"`
}

type RegisteredEntry struct {
	Name        string `json:"name"`
	CMKeyExact  string `json:"cmKeyExact,omitempty"`
	CMKeyPrefix string `json:"cmKeyPrefix,omitempty"`
	EnvVar      string `json:"envVar,omitempty"`
}

var envHelperCall = regexp.MustCompile(`env\.(StringFromEnv|ParseDurationFromEnv|ParseBoolFromEnv|StringsFromEnv|ParseNumFromEnv|ParseInt64FromEnv|ParseFloatFromEnv|ParseFloat64FromEnv|ParseStringToStringFromEnv)\(`)

// flagVarSel matches pflag/cobra binding methods whose default argument may be an
// env read, e.g. StringVar, IntVar, DurationVar, StringSliceVar, StringVarP, and
// helpers like cli.BoundedFloat64Var.
var flagVarSel = regexp.MustCompile(`Var[P]?$`)

func main() {
	repoRoot := flag.String("repo-root", ".", "path to argo-cd repository root")
	outPath := flag.String("out", "", "write JSON inventory to this path (stdout if empty)")
	allowlistOut := flag.String("allowlist-out", "", "write uncovered CM keys + env vars as an allowlist file")
	flagAllowlistOut := flag.String("flag-allowlist-out", "", "write uncovered pure flags as an allowlist file")
	flag.Parse()

	inv := &Inventory{}
	must(collectSettingsCMKeys(filepath.Join(*repoRoot, "util/settings/settings.go"), inv))
	must(collectEnvVars(*repoRoot, inv))
	must(collectFlags(*repoRoot, inv))
	must(collectCmdParamRefs(filepath.Join(*repoRoot, "manifests/base"), inv))
	must(collectDocCMKeys(filepath.Join(*repoRoot, "docs/operator-manual/argocd-cm.yaml"), inv))
	must(collectDocCMKeys(filepath.Join(*repoRoot, "docs/operator-manual/argocd-cmd-params-cm.yaml"), inv))
	must(collectDocCMKeys(filepath.Join(*repoRoot, "docs/operator-manual/argocd-rbac-cm.yaml"), inv))

	sort.Strings(inv.CMKeys)
	inv.CMKeys = unique(inv.CMKeys)
	sort.Slice(inv.EnvVars, func(i, j int) bool { return inv.EnvVars[i].Name < inv.EnvVars[j].Name })
	sort.Slice(inv.CmdParams, func(i, j int) bool { return inv.CmdParams[i].CMKey < inv.CmdParams[j].CMKey })
	sort.Slice(inv.Flags, func(i, j int) bool {
		if inv.Flags[i].Component != inv.Flags[j].Component {
			return inv.Flags[i].Component < inv.Flags[j].Component
		}
		return inv.Flags[i].Name < inv.Flags[j].Name
	})
	inv.Flags = uniqueFlags(inv.Flags)

	// Optional: import registry coverage if the package is available via a side file.
	// Coverage is computed by the completeness test; here we only emit discovery.

	data, err := json.MarshalIndent(inv, "", "  ")
	must(err)
	if *outPath == "" {
		fmt.Println(string(data))
	} else {
		must(os.WriteFile(*outPath, data, 0o644))
		fmt.Fprintf(os.Stderr, "wrote %s (%d cm keys, %d env vars, %d cmd-param refs, %d flags)\n",
			*outPath, len(inv.CMKeys), len(inv.EnvVars), len(inv.CmdParams), len(inv.Flags))
	}

	if *allowlistOut != "" {
		var lines []string
		lines = append(lines, "# Auto-generated uncovered keys allowlist (shrink as registry grows)")
		lines = append(lines, "# Format: cm:<key> or env:<NAME>")
		lines = append(lines, "# Only UNcovered sources are listed; keys claimed by a registry descriptor are omitted.")
		for _, k := range inv.CMKeys {
			if configbus.DescriptorCoversCMKey(k) {
				continue
			}
			lines = append(lines, "cm:"+k)
		}
		// Only genuinely standalone env reads are tracked as env sources. An env
		// var is standalone when it has at least one read that is not a flag
		// default fallback; flag-bound-only vars are documented via their
		// argocd-cmd-params-cm key instead.
		standalone := map[string]bool{}
		for _, e := range inv.EnvVars {
			if !e.FlagBound {
				standalone[e.Name] = true
			}
		}
		seenEnv := map[string]bool{}
		for _, e := range inv.EnvVars {
			if !standalone[e.Name] || seenEnv[e.Name] {
				continue
			}
			seenEnv[e.Name] = true
			if configbus.DescriptorCoversEnv(e.Name) {
				continue
			}
			lines = append(lines, "env:"+e.Name)
		}
		// Cmd-param env vars are just the transport for their CM key; track the CM
		// key only.
		for _, p := range inv.CmdParams {
			if configbus.DescriptorCoversCMKey(p.CMKey) {
				continue
			}
			lines = append(lines, "cm:"+p.CMKey)
		}
		lines = unique(lines)
		sort.Strings(lines)
		must(os.WriteFile(*allowlistOut, []byte(strings.Join(lines, "\n")+"\n"), 0o644))
		fmt.Fprintf(os.Stderr, "wrote allowlist %s (%d lines)\n", *allowlistOut, len(lines))
	}

	if *flagAllowlistOut != "" {
		var lines []string
		lines = append(lines, "# Auto-generated uncovered pure-flag allowlist (shrink as registry grows)")
		lines = append(lines, "# Format: flag:<component>:<name>")
		lines = append(lines, "# Pure flags have no env/CM transport; they need SourceFlagOnly descriptors.")
		for _, f := range inv.Flags {
			if !f.PureFlag {
				continue
			}
			entry := "flag:" + f.Component + ":" + f.Name
			if configbus.DescriptorCoversFlag(f.Name) {
				continue
			}
			lines = append(lines, entry)
		}
		lines = unique(lines)
		sort.Strings(lines)
		must(os.WriteFile(*flagAllowlistOut, []byte(strings.Join(lines, "\n")+"\n"), 0o644))
		fmt.Fprintf(os.Stderr, "wrote flag allowlist %s (%d lines)\n", *flagAllowlistOut, len(lines))
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func collectSettingsCMKeys(path string, inv *Inventory) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range vs.Names {
			_ = name
			if i >= len(vs.Values) {
				continue
			}
			bl, ok := vs.Values[i].(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				continue
			}
			val := strings.Trim(bl.Value, `"`)
			if looksLikeCMKey(val) {
				inv.CMKeys = append(inv.CMKeys, val)
			}
		}
		return true
	})
	return nil
}

func looksLikeCMKey(s string) bool {
	if s == "" {
		return false
	}
	// Reject label selectors, URLs, and other non-key noise.
	if strings.ContainsAny(s, " /=") || strings.Contains(s, "://") {
		return false
	}
	// Heuristic: dotted keys or known single-token settings keys.
	if strings.Contains(s, ".") {
		return true
	}
	switch s {
	case "url", "passwordPattern", "additionalUrls", "admin.enabled", "scopes", "installationID", "globalProjects":
		return true
	default:
		return false
	}
}

func collectEnvVars(repoRoot string, inv *Inventory) error {
	skip := []string{"/vendor/", "/testdata/", "/.git/", "/ui/", "/node_modules/", "/test/"}
	return filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == "vendor" || base == "testdata" || base == ".git" || base == "ui" || base == "node_modules" || base == "test" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		for _, s := range skip {
			if strings.Contains(path, s) {
				return nil
			}
		}
		// Client-side packages excluded from "runtime" inventory classification.
		if strings.HasPrefix(rel, "cmd/argocd/") || strings.HasPrefix(rel, "pkg/apiclient/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !envHelperCall.Match(data) {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, data, 0)
		if err != nil {
			return nil // skip unparseable
		}
		pkg := f.Name.Name
		flagBound := flagBoundEnvCalls(f)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !isEnvFromEnvCall(call) {
				return true
			}
			if len(call.Args) < 1 {
				return true
			}
			name, ok := stringLit(call.Args[0])
			if !ok {
				return true
			}
			def := ""
			if len(call.Args) > 1 {
				def, _ = stringLit(call.Args[1])
			}
			helper := ""
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				helper = sel.Sel.Name
			}
			inv.EnvVars = append(inv.EnvVars, EnvVar{
				Name:      name,
				Default:   def,
				Helper:    helper,
				Package:   pkg,
				File:      rel,
				FlagBound: flagBound[call],
			})
			return true
		})
		return nil
	})
}

// isEnvFromEnvCall reports whether call is an env.<Something>FromEnv(...) helper.
func isEnvFromEnvCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Name != "env" {
		return false
	}
	return strings.Contains(sel.Sel.Name, "FromEnv")
}

// isFlagBindingCall reports whether call is a pflag/cobra flag binding whose
// default argument (possibly nested) may be an env read. Matches selector calls
// ending in Var/VarP (StringVar, IntVar, DurationVar, StringSliceVar, ...) and
// helpers like cli.BoundedFloat64Var.
func isFlagBindingCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return flagVarSel.MatchString(sel.Sel.Name)
}

// flagBoundEnvCalls returns the set of env.<...>FromEnv call nodes that appear
// (at any depth) within a flag binding call, i.e. env reads used only as flag
// defaults. These are the transport for argocd-cmd-params-cm keys rather than
// standalone env reads.
func flagBoundEnvCalls(f *ast.File) map[*ast.CallExpr]bool {
	bound := map[*ast.CallExpr]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isFlagBindingCall(call) {
			return true
		}
		for _, arg := range call.Args {
			ast.Inspect(arg, func(m ast.Node) bool {
				if ec, ok := m.(*ast.CallExpr); ok && isEnvFromEnvCall(ec) {
					bound[ec] = true
				}
				return true
			})
		}
		return true
	})
	return bound
}

func stringLit(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return strings.Trim(v.Value, `"`), true
		}
	case *ast.Ident:
		// Skip non-literal (const refs) — still record name unknown
		return "", false
	}
	return "", false
}

func collectCmdParamRefs(baseDir string, inv *Inventory) error {
	if _, err := os.Stat(baseDir); err != nil {
		return nil
	}
	return filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		if !strings.Contains(text, "argocd-cmd-params-cm") && !strings.Contains(text, "argocd-cm") && !strings.Contains(text, "argocd-rbac-cm") {
			return nil
		}
		var docs []any
		dec := yaml.NewDecoder(strings.NewReader(text))
		for {
			var doc any
			if err := dec.Decode(&doc); err != nil {
				break
			}
			docs = append(docs, doc)
		}
		rel := path
		for _, doc := range docs {
			findConfigMapKeyRefs(doc, rel, inv)
		}
		return nil
	})
}

func findConfigMapKeyRefs(v any, file string, inv *Inventory) {
	switch t := v.(type) {
	case map[string]any:
		if envName, ok := t["name"].(string); ok {
			if vf, ok := t["valueFrom"].(map[string]any); ok {
				if cmr, ok := vf["configMapKeyRef"].(map[string]any); ok {
					cmName, _ := cmr["name"].(string)
					key, _ := cmr["key"].(string)
					if key != "" && (strings.Contains(cmName, "cmd-params") || cmName == "argocd-cm" || cmName == "argocd-rbac-cm") {
						inv.CmdParams = append(inv.CmdParams, CmdParamRef{CMKey: key, EnvVar: envName, File: file})
						inv.CMKeys = append(inv.CMKeys, key)
					}
				}
			}
		}
		for _, child := range t {
			findConfigMapKeyRefs(child, file, inv)
		}
	case []any:
		for _, child := range t {
			findConfigMapKeyRefs(child, file, inv)
		}
	}
}

func collectDocCMKeys(path string, inv *Inventory) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	dataMap, _ := doc["data"].(map[string]any)
	for k := range dataMap {
		if looksLikeCMKey(k) {
			inv.CMKeys = append(inv.CMKeys, k)
		}
	}
	return nil
}

func uniqueFlags(in []FlagRef) []FlagRef {
	seen := map[string]bool{}
	var out []FlagRef
	for _, f := range in {
		key := f.Component + ":" + f.Name + ":" + f.File
		if f.Name == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}

func inferComponentFromPath(rel string) string {
	switch {
	case strings.Contains(rel, "argocd-application-controller"):
		return "controller"
	case strings.Contains(rel, "argocd-repo-server"):
		return "reposerver"
	case strings.Contains(rel, "argocd-server"):
		return "server"
	case strings.Contains(rel, "argocd-applicationset"):
		return "applicationset"
	case strings.Contains(rel, "argocd-notification"):
		return "notifications"
	case strings.Contains(rel, "argocd-commit-server"):
		return "commitserver"
	case strings.Contains(rel, "argocd-dex"):
		return "dex"
	default:
		return "shared"
	}
}

// collectFlags discovers cobra/pflag *Var / *VarP bindings under cmd/.
func collectFlags(repoRoot string, inv *Inventory) error {
	cmdRoot := filepath.Join(repoRoot, "cmd")
	if _, err := os.Stat(cmdRoot); err != nil {
		return nil
	}
	return filepath.Walk(cmdRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		if strings.HasPrefix(rel, "cmd/argocd/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, data, 0)
		if err != nil {
			return nil
		}
		component := inferComponentFromPath(rel)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isFlagBindingCall(call) {
				return true
			}
			name, envVar, hasEnv := flagBindingMeta(call)
			if name == "" {
				return true
			}
			inv.Flags = append(inv.Flags, FlagRef{
				Name:      name,
				Component: component,
				File:      rel,
				HasEnv:    hasEnv,
				EnvVar:    envVar,
				PureFlag:  !hasEnv,
			})
			return true
		})
		return nil
	})
}

// flagBindingMeta extracts the flag name and optional env var from a *Var/*VarP call.
func flagBindingMeta(call *ast.CallExpr) (name, envVar string, hasEnv bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	nameIdx := 1
	if sel.Sel.Name == "BoundedFloat64Var" {
		nameIdx = 2
	}
	if len(call.Args) <= nameIdx {
		return "", "", false
	}
	name, ok = stringLit(call.Args[nameIdx])
	if !ok || name == "" {
		return "", "", false
	}
	for _, arg := range call.Args {
		ast.Inspect(arg, func(m ast.Node) bool {
			ec, ok := m.(*ast.CallExpr)
			if !ok || !isEnvFromEnvCall(ec) {
				return true
			}
			hasEnv = true
			if len(ec.Args) > 0 {
				if ev, ok := stringLit(ec.Args[0]); ok {
					envVar = ev
				}
			}
			return false
		})
	}
	return name, envVar, hasEnv
}
