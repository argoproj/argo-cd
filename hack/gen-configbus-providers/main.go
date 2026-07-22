// Command gen-configbus-providers generates notConfiguredProvider, ChainProvider,
// and StaticProvider from the configbus.Provider interface.
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
// and left as ErrNotConfigured / no-ops on Static / notConfigured.
var specialMethods = map[string]bool{
	"Subscribe":   true,
	"Unsubscribe": true,
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

	write(filepath.Join(root, "util", "configbus", "zz_generated.not_configured.go"), genNotConfigured(methods))
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
// not use the context yet (Static / notConfigured leaves).
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

const generatedImports = `import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)
`

func genNotConfigured(methods []method) string {
	var b strings.Builder
	b.WriteString(`// Code generated by hack/gen-configbus-providers. DO NOT EDIT.

package configbus

`)
	b.WriteString(generatedImports)
	b.WriteString(`
// notConfiguredProvider returns ErrNotConfigured for every field getter and
// no-ops lifecycle methods. Leaf providers embed it and override owned methods.
type notConfiguredProvider struct{}

`)
	for _, m := range methods {
		writeNotConfiguredMethod(&b, m)
	}
	return b.String()
}

func writeNotConfiguredMethod(b *strings.Builder, m method) {
	recv := "(notConfiguredProvider)"
	params := unusedParams(m.Params)
	switch {
	case m.Name == "Subscribe" || m.Name == "Unsubscribe":
		fmt.Fprintf(b, "func %s %s(%s) {}\n\n", recv, m.Name, params)
	case m.IsField || (m.HasError && m.ResultType != ""):
		zero := zeroValue(m.ResultType)
		fmt.Fprintf(b, "func %s %s(%s) %s {\n\treturn %s, ErrNotConfigured\n}\n\n", recv, m.Name, params, m.Results, zero)
	case m.HasError:
		fmt.Fprintf(b, "func %s %s(%s) %s {\n\treturn ErrNotConfigured\n}\n\n", recv, m.Name, params, m.Results)
	default:
		fmt.Fprintf(b, "func %s %s(%s)%s {}\n\n", recv, m.Name, params, resultSuffix(m.Results))
	}
}

func genChain(methods []method) string {
	var b strings.Builder
	b.WriteString(`// Code generated by hack/gen-configbus-providers. DO NOT EDIT.

package configbus

`)
	b.WriteString(generatedImports)
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
	default:
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
	b.WriteString(generatedImports)
	b.WriteString(`
// StaticFields holds in-memory nilable config values for StaticProvider.
// Construct a literal with only the fields this call site owns; unset fields
// return ErrNotConfigured so ChainProvider can fall through to later links.
//
// Field rules:
//   - Method returning (T, error) where T is not a pointer → field *T
//   - Method returning (*U, error) → field **U (nil outer = unset; outer set with
//     nil inner = configured nil)
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
	notConfiguredProvider
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

func resultSuffix(results string) string {
	if results == "" {
		return ""
	}
	return " " + results
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
