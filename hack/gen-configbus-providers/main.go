// Command gen-configbus-providers generates ChainProvider and StaticProvider
// from the configbus.Provider interface.
//
// Usage (from repo root):
//
//	go run ./hack/gen-configbus-providers
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const providerPath = "util/configbus/provider.go"

// Non-field methods: lifecycle / escape hatches routed specially by ChainProvider
// and left as no-ops on leaves via the embedded empty ChainProvider.
var specialMethods = map[string]bool{
	"Configuration":   true,
	"Subscribe":       true,
	"Unsubscribe":     true,
	"SubscribeCRD":    true,
	"UnsubscribeCRD":  true,
}

// aliasMethods maps deprecated/alternate getters onto a canonical shared getter.
// Aliases are not StaticFields; Static and Chain route them to the canonical method.
var aliasMethods = map[string]string{
	"NotificationsApplicationNamespaces": "ApplicationNamespaces",
}

type method struct {
	Name       string
	Params     string // inside parens, e.g. "ctx context.Context"
	Results    string // e.g. "(time.Duration, error)" or empty for no results
	ResultType string // first result type without error, empty if none / only error
	HasError   bool
	IsField    bool // true if (T, error) getter eligible for Static / firstConfigured
}

func main() {
	root := findRoot()
	methods, _ := parseProvider(filepath.Join(root, providerPath))

	write(filepath.Join(root, "util", "configbus", "zz_generated.chain_provider.go"), genChain(methods))
	write(filepath.Join(root, "util", "configbus", "zz_generated.static_provider.go"), genStatic(methods))
	fmt.Println("generated configbus providers from", providerPath)
}

func findRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, providerPath)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fatal(fmt.Errorf("could not find %s walking up from %s", providerPath, wd))
		}
		dir = parent
	}
}

func parseProvider(path string) ([]method, map[string]string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		fatal(err)
	}
	imports := map[string]string{}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		switch path {
		case "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1":
			name = "v1alpha1"
		case "github.com/argoproj/argo-cd/v3/util/settings":
			name = "settings"
		case "k8s.io/apimachinery/pkg/util/wait":
			name = "wait"
		case "context":
			name = "context"
		}
		imports[name] = path
	}

	var methods []method
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Provider" {
				continue
			}
			iface, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			for _, field := range iface.Methods.List {
				ft, ok := field.Type.(*ast.FuncType)
				if !ok || len(field.Names) != 1 {
					continue
				}
				name := field.Names[0].Name
				m := method{Name: name}
				if ft.Params != nil {
					m.Params = joinFields(ft.Params.List)
				}
				if ft.Results != nil {
					m.Results = "(" + joinFields(ft.Results.List) + ")"
					results := ft.Results.List
					if len(results) == 2 && exprString(results[1].Type) == "error" {
						m.HasError = true
						m.ResultType = exprString(results[0].Type)
						m.IsField = !specialMethods[name]
					} else if len(results) == 1 && exprString(results[0].Type) == "error" {
						m.HasError = true
					}
				} else {
					m.Results = ""
				}
				if specialMethods[name] {
					m.IsField = false
				}
				if _, ok := aliasMethods[name]; ok {
					m.IsField = false
				}
				methods = append(methods, m)
			}
		}
	}
	if len(methods) == 0 {
		fatal(fmt.Errorf("no methods found on Provider in %s", path))
	}
	return methods, imports
}

func joinFields(fields []*ast.Field) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		typ := exprString(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typ)
			continue
		}
		names := make([]string, len(f.Names))
		for i, n := range f.Names {
			names[i] = n.Name
		}
		parts = append(parts, strings.Join(names, ", ")+" "+typ)
	}
	return strings.Join(parts, ", ")
}

func exprString(e ast.Expr) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), e); err != nil {
		fatal(err)
	}
	return buf.String()
}

// unusedParams renames context parameters to "_" for implementations that do
// not use the context yet (Static leaf getters).
func unusedParams(params string) string {
	return strings.ReplaceAll(params, "ctx context.Context", "_ context.Context")
}

// callArgs returns the argument list to forward (parameter names only).
func callArgs(params string) string {
	if params == "" {
		return ""
	}
	parts := strings.Split(params, ", ")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// "ctx context.Context" or "subCh chan<- *settings.ArgoCDSettings"
		fields := strings.Fields(p)
		if len(fields) == 0 {
			continue
		}
		names = append(names, fields[0])
	}
	return strings.Join(names, ", ")
}

func genImportBlock(methods []method) string {
	need := map[string]bool{
		"time":        false,
		"wait":        false,
		"resource":    false,
		"v1alpha1":    false,
		"normalizers": false,
		"settings":    false,
	}
	for _, m := range methods {
		blob := m.Params + " " + m.Results + " " + m.ResultType
		if strings.Contains(blob, "time.") {
			need["time"] = true
		}
		if strings.Contains(blob, "wait.") {
			need["wait"] = true
		}
		if strings.Contains(blob, "resource.") {
			need["resource"] = true
		}
		if strings.Contains(blob, "v1alpha1.") {
			need["v1alpha1"] = true
		}
		if strings.Contains(blob, "normalizers.") {
			need["normalizers"] = true
		}
		if strings.Contains(blob, "settings.") {
			need["settings"] = true
		}
	}
	var b strings.Builder
	b.WriteString("import (\n")
	b.WriteString("\t\"context\"\n")
	if need["time"] {
		b.WriteString("\t\"time\"\n\n")
	}
	if need["resource"] {
		b.WriteString("\t\"k8s.io/apimachinery/pkg/api/resource\"\n")
	}
	if need["wait"] {
		b.WriteString("\t\"k8s.io/apimachinery/pkg/util/wait\"\n")
	}
	if need["resource"] || need["wait"] {
		b.WriteString("\n")
	}
	if need["v1alpha1"] {
		b.WriteString("\t\"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1\"\n")
	}
	if need["normalizers"] {
		b.WriteString("\t\"github.com/argoproj/argo-cd/v3/util/argo/normalizers\"\n")
	}
	if need["settings"] {
		b.WriteString("\t\"github.com/argoproj/argo-cd/v3/util/settings\"\n")
	}
	b.WriteString(")\n\n")
	return b.String()
}

func genChain(methods []method) string {
	var b strings.Builder
	b.WriteString(`// Code generated by hack/gen-configbus-providers. DO NOT EDIT.

package configbus

`)
	b.WriteString(genImportBlock(methods))
	b.WriteString(`
// ChainProvider tries each link in order. The first result that is not
// ErrNotConfigured wins. Non-field methods are routed explicitly.
type ChainProvider struct {
	links []Provider
}

// NewChainProvider constructs a ChainProvider. Nil links are skipped.
func NewChainProvider(links ...Provider) *ChainProvider {
	out := make([]Provider, 0, len(links))
	for _, l := range links {
		if l != nil {
			out = append(out, l)
		}
	}
	return &ChainProvider{links: out}
}

// Ensure ChainProvider implements Provider.
var _ Provider = (*ChainProvider)(nil)

`)
	for _, m := range methods {
		writeChainMethod(&b, m)
	}
	return b.String()
}

func writeChainMethod(b *strings.Builder, m method) {
	switch m.Name {
	case "Subscribe":
		b.WriteString(`func (c *ChainProvider) Subscribe(subCh chan<- *settings.ArgoCDSettings) {
	for _, l := range c.links {
		l.Subscribe(subCh)
	}
}

`)
	case "Unsubscribe":
		b.WriteString(`func (c *ChainProvider) Unsubscribe(subCh chan<- *settings.ArgoCDSettings) {
	for _, l := range c.links {
		l.Unsubscribe(subCh)
	}
}

`)
	case "SubscribeCRD":
		b.WriteString(`func (c *ChainProvider) SubscribeCRD(subCh chan<- struct{}) {
	for _, l := range c.links {
		l.SubscribeCRD(subCh)
	}
}

`)
	case "UnsubscribeCRD":
		b.WriteString(`func (c *ChainProvider) UnsubscribeCRD(subCh chan<- struct{}) {
	for _, l := range c.links {
		l.UnsubscribeCRD(subCh)
	}
}

`)
	case "Configuration":
		fmt.Fprintf(b, `func (c *ChainProvider) Configuration(%s) %s {
	return firstConfigured(func(p Provider) %s {
		return p.Configuration(%s)
	}, c.links)
}

`, m.Params, m.Results, m.Results, callArgs(m.Params))
	default:
		if canon, ok := aliasMethods[m.Name]; ok {
			fmt.Fprintf(b, `// %s is an alias for %s for call sites that predate the shared getter.
func (c *ChainProvider) %s(%s) %s {
	return c.%s(%s)
}

`, m.Name, canon, m.Name, m.Params, m.Results, canon, callArgs(m.Params))
			return
		}
		if !m.IsField {
			return
		}
		fmt.Fprintf(b, `func (c *ChainProvider) %s(%s) %s {
	return firstConfigured(func(p Provider) %s {
		return p.%s(%s)
	}, c.links)
}

`, m.Name, m.Params, m.Results, m.Results, m.Name, callArgs(m.Params))
	}
}

func genStatic(methods []method) string {
	var b strings.Builder
	b.WriteString(`// Code generated by hack/gen-configbus-providers. DO NOT EDIT.

package configbus

`)
	b.WriteString(genImportBlock(methods))
	b.WriteString(`
// StaticFields holds in-memory nilable config values for StaticProvider.
// Construct a literal with only the fields this call site owns; unset fields
// return ErrNotConfigured so ChainProvider can fall through to later links.
//
// Field rules:
//   - Method returning (T, error) where T is not a pointer → field *T
//   - Method returning (*U, error) → field **U (nil outer = unset; outer set with
//     nil inner = configured nil). Prefer returning a value type (or a small
//     policy struct) instead of *U when “configured nil” is a product meaning.
//   - Method returning ([]T, error) or (map[K]V, error) → field *[]T / *map[K]V
type StaticFields struct {
`)
	for _, m := range methods {
		if !m.IsField {
			continue
		}
		fmt.Fprintf(&b, "\t%s %s\n", m.Name, staticFieldType(m.ResultType))
	}
	b.WriteString(`}

// StaticProvider is a leaf Provider backed by StaticFields.
type StaticProvider struct {
	// ChainProvider is embedded with no links on purpose: an empty chain
	// resolves every promoted field getter to ErrNotConfigured and makes the
	// lifecycle methods no-ops, so this leaf only implements the fields it
	// owns. Do not populate its links.
	ChainProvider
	Fields StaticFields
}

// Ensure StaticProvider implements Provider.
var _ Provider = (*StaticProvider)(nil)

`)

	for _, m := range methods {
		if !m.IsField {
			continue
		}
		writeStaticGetter(&b, m)
	}
	for _, m := range methods {
		if canon, ok := aliasMethods[m.Name]; ok {
			fmt.Fprintf(&b, `// %s is an alias for %s for call sites that predate the shared getter.
func (p *StaticProvider) %s(%s) %s {
	return p.%s(%s)
}

`, m.Name, canon, m.Name, m.Params, m.Results, canon, callArgs(m.Params))
		}
	}
	return b.String()
}

func writeStaticGetter(b *strings.Builder, m method) {
	field := m.Name
	zero := zeroValue(m.ResultType)
	params := unusedParams(m.Params)
	fmt.Fprintf(b, `func (p *StaticProvider) %s(%s) %s {
	if p == nil || p.Fields.%s == nil {
		return %s, ErrNotConfigured
	}
	return *p.Fields.%s, nil
}

`, m.Name, params, m.Results, field, zero, field)
}

func staticFieldType(resultType string) string {
	return "*" + resultType
}

func zeroValue(typ string) string {
	switch {
	case typ == "string":
		return `""`
	case typ == "bool":
		return "false"
	case typ == "int" || typ == "int32" || typ == "int64":
		return "0"
	case typ == "time.Duration":
		return "0"
	case strings.HasPrefix(typ, "*") || strings.HasPrefix(typ, "[]") || strings.HasPrefix(typ, "map["):
		return "nil"
	case typ == "settings.ArgoCDDiffOptions":
		return "settings.ArgoCDDiffOptions{}"
	default:
		return typ + "{}"
	}
}

func write(path, src string) {
	formatted, err := format.Source([]byte(src))
	if err != nil {
		_ = os.WriteFile(path, []byte(src), 0o644)
		fatal(fmt.Errorf("format %s: %w", path, err))
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "gen-configbus-providers: %v\n", err)
	os.Exit(1)
}
