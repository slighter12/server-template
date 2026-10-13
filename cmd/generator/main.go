// Command generator produces OpenTelemetry tracing proxies for Go interfaces.
//
// Usage from the repository root:
//
//	go run ./cmd/generator --config=.generator.yaml
package main

import (
	"bytes"
	"cmp"
	_ "embed"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

//go:embed templates/otel_proxy.tmpl
var otelProxyTmpl string

var templateMap = map[string]string{
	"otel": otelProxyTmpl,
}

// Method describes one interface method.
type Method struct {
	Name       string
	Params     string
	CallParams string
	Results    []string
	ResultSig  string // " T", " (A, B)" or ""
	Assign     string // "ret0, err := "
	ReturnVars string // "ret0, err"
	CtxName    string
	HasError   bool
	// Traced is false when the first parameter is not a context.Context;
	// such methods are passed through without a span.
	Traced bool
}

// TemplateData is the input of the proxy templates.
type TemplateData struct {
	PackageName   string
	SourcePackage string
	ProxyName     string
	InterfaceName string
	TracerName    string
	Methods       []Method
	ImportGroups  [][]string
}

// Config describes one proxy to generate.
type Config struct {
	Source     string `yaml:"source"`
	Output     string `yaml:"output"`
	Interface  string `yaml:"interface"`
	Package    string `yaml:"package"`
	Tracer     string `yaml:"tracer"`
	Template   string `yaml:"template"`
	ModuleName string `yaml:"moduleName"`
}

// FileConfig is the .generator.yaml layout.
type FileConfig struct {
	Generators []Config `yaml:"generators"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generator:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := pflag.String("config", "", "Path to config file")
	var c Config
	pflag.StringVar(&c.Source, "source", "", "Source file containing the interface")
	pflag.StringVar(&c.Output, "output", "proxy.gen.go", "Output file for the generated proxy")
	pflag.StringVar(&c.Interface, "interface", "", "Interface to generate proxy for")
	pflag.StringVar(&c.Package, "package", "", "Package name for the generated proxy")
	pflag.StringVar(&c.Tracer, "tracer", "", "Tracer name for the generated proxy")
	pflag.StringVar(&c.Template, "template", "", "Template type (e.g. otel)")
	pflag.StringVar(&c.ModuleName, "module-name", "", "Module name for import paths")
	pflag.Parse()

	configs := []Config{c}
	if *configPath != "" {
		data, err := os.ReadFile(*configPath)
		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}
		var fc FileConfig
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return fmt.Errorf("parse config: %w", err)
		}
		configs = fc.Generators
	}

	for _, cfg := range configs {
		src, err := generate(cfg)
		if err != nil {
			return fmt.Errorf("generate %s: %w", cfg.Interface, err)
		}
		if err := os.WriteFile(cfg.Output, src, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", cfg.Output, err)
		}
		fmt.Printf("Proxy for interface '%s' generated at '%s' using template '%s'\n", cfg.Interface, cfg.Output, cfg.Template)
	}

	return nil
}

// generate returns the gofmt-formatted proxy source for cfg.
func generate(cfg Config) ([]byte, error) {
	if cfg.Source == "" || cfg.Interface == "" || cfg.Template == "" || cfg.ModuleName == "" {
		return nil, errors.New("source, interface, template and module-name must be specified")
	}

	tmplText, ok := templateMap[cfg.Template]
	if !ok {
		return nil, fmt.Errorf("unsupported template type: %s. Supported types: %v", cfg.Template, getSupportedTemplates())
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, cfg.Source, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse source: %w", err)
	}

	methods, used, err := extractMethods(node, cfg.Interface)
	if err != nil {
		return nil, err
	}

	srcImport, err := sourceImportPath(cfg.Source, cfg.ModuleName)
	if err != nil {
		return nil, err
	}

	pkg := cfg.Package
	if pkg == "" {
		pkg = node.Name.Name
	}

	data := &TemplateData{
		PackageName:   pkg,
		SourcePackage: node.Name.Name,
		ProxyName:     cfg.Interface + "Proxy",
		InterfaceName: cfg.Interface,
		TracerName:    cfg.Tracer,
		Methods:       methods,
		ImportGroups:  groupImports(buildImports(node, used, srcImport, cfg.ModuleName), cfg.ModuleName),
	}

	tmpl, err := template.New("proxy").Parse(tmplText)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	out, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated code: %w\n%s", err, buf.String())
	}

	return out, nil
}

// sourceImportPath derives the import path of the package holding source from the nearest go.mod.
func sourceImportPath(source, moduleName string) (string, error) {
	abs, err := filepath.Abs(filepath.Dir(source))
	if err != nil {
		return "", err
	}
	for dir := abs; ; dir = filepath.Dir(dir) {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
			mod := moduleName
			for line := range strings.SplitSeq(string(data), "\n") {
				if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
					mod = strings.Trim(strings.TrimSpace(rest), `"`)
					break
				}
			}
			rel, err := filepath.Rel(dir, abs)
			if err != nil {
				return "", err
			}

			return path.Join(mod, filepath.ToSlash(rel)), nil
		}
		if dir == filepath.Dir(dir) {
			return "", fmt.Errorf("go.mod not found above %s", source)
		}
	}
}

// buildImports returns import specs (`alias "path"` or `"path"`) for everything the proxy needs.
func buildImports(node *ast.File, used map[string]bool, srcImport, moduleName string) []string {
	byPath := map[string]string{} // path -> spec

	add := func(p, alias string) {
		if alias != "" {
			byPath[p] = alias + " " + strconv.Quote(p)
		} else {
			byPath[p] = strconv.Quote(p)
		}
	}

	for _, p := range []string{
		"fmt",
		"go.opentelemetry.io/otel",
		"go.opentelemetry.io/otel/attribute",
		"go.opentelemetry.io/otel/codes",
		"go.opentelemetry.io/otel/trace",
		moduleName + "/internal/pkg/apperr",
	} {
		add(p, "")
	}
	add(srcImport, "")

	for _, imp := range node.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		name, alias := path.Base(p), ""
		if imp.Name != nil {
			name, alias = imp.Name.Name, imp.Name.Name
		}
		if used[name] {
			add(p, alias)
		}
	}

	return slices.Sorted(maps.Values(byPath))
}

func importPathOf(spec string) string {
	i := strings.IndexByte(spec, '"')
	p, _ := strconv.Unquote(spec[i:])

	return p
}

// groupImports splits imports into std / third-party / module-local groups.
func groupImports(specs []string, moduleName string) [][]string {
	var std, third, local []string
	for _, s := range specs {
		p := importPathOf(s)
		switch {
		case p == moduleName || strings.HasPrefix(p, moduleName+"/"):
			local = append(local, s)
		case !strings.Contains(strings.SplitN(p, "/", 2)[0], "."):
			std = append(std, s)
		default:
			third = append(third, s)
		}
	}
	sortSpecs := func(l []string) {
		slices.SortFunc(l, func(a, b string) int { return cmp.Compare(importPathOf(a), importPathOf(b)) })
	}
	var groups [][]string
	for _, g := range [][]string{std, third, local} {
		if len(g) > 0 {
			sortSpecs(g)
			groups = append(groups, g)
		}
	}

	return groups
}

func extractMethods(node *ast.File, interfaceName string) ([]Method, map[string]bool, error) {
	var iface *ast.InterfaceType
	for _, decl := range node.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == interfaceName {
				if iface, ok = ts.Type.(*ast.InterfaceType); !ok {
					return nil, nil, fmt.Errorf("%s is not an interface", interfaceName)
				}
			}
		}
	}
	if iface == nil {
		return nil, nil, fmt.Errorf("interface %s not found", interfaceName)
	}

	used := map[string]bool{}
	var methods []Method
	for _, m := range iface.Methods.List {
		ft, ok := m.Type.(*ast.FuncType)
		if !ok || len(m.Names) == 0 {
			return nil, nil, fmt.Errorf("embedded interfaces are not supported (in %s)", interfaceName)
		}
		methods = append(methods, buildMethod(m.Names[0].Name, ft, used))
	}

	return methods, used, nil
}

func buildMethod(name string, ft *ast.FuncType, used map[string]bool) Method {
	m := Method{Name: name, CtxName: "ctx"}

	var params, calls []string
	if ft.Params != nil {
		idx := 0
		for _, f := range ft.Params.List {
			t := formatType(f.Type, used)
			_, variadic := f.Type.(*ast.Ellipsis)
			names := f.Names
			if len(names) == 0 {
				names = []*ast.Ident{nil}
			}
			for _, n := range names {
				pn := fmt.Sprintf("arg%d", idx)
				if n != nil && n.Name != "_" {
					pn = n.Name
				}
				if idx == 0 && t == "context.Context" {
					m.Traced = true
					if n == nil || n.Name == "_" {
						pn = "ctx"
					}
					m.CtxName = pn
				}
				params = append(params, pn+" "+t)
				if variadic {
					calls = append(calls, pn+"...")
				} else {
					calls = append(calls, pn)
				}
				idx++
			}
		}
	}
	m.Params = strings.Join(params, ", ")
	m.CallParams = strings.Join(calls, ", ")

	if ft.Results != nil {
		for _, f := range ft.Results.List {
			t := formatType(f.Type, used)
			for range max(len(f.Names), 1) {
				m.Results = append(m.Results, t)
			}
		}
	}
	m.HasError = len(m.Results) > 0 && m.Results[len(m.Results)-1] == "error"

	switch len(m.Results) {
	case 0:
	case 1:
		m.ResultSig = " " + m.Results[0]
	default:
		m.ResultSig = " (" + strings.Join(m.Results, ", ") + ")"
	}

	if m.HasError {
		vars := make([]string, len(m.Results))
		for i := range vars {
			vars[i] = fmt.Sprintf("ret%d", i)
		}
		vars[len(vars)-1] = "err"
		m.ReturnVars = strings.Join(vars, ", ")
		m.Assign = m.ReturnVars + " := "
	}

	return m
}

// formatType renders a type expression and records the package qualifiers it uses.
func formatType(expr ast.Expr, used map[string]bool) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + formatType(t.X, used)
	case *ast.SelectorExpr:
		x := formatType(t.X, used)
		used[x] = true

		return x + "." + t.Sel.Name
	case *ast.ArrayType:
		l := ""
		if t.Len != nil {
			l = types.ExprString(t.Len)
		}

		return "[" + l + "]" + formatType(t.Elt, used)
	case *ast.MapType:
		return "map[" + formatType(t.Key, used) + "]" + formatType(t.Value, used)
	case *ast.Ellipsis:
		return "..." + formatType(t.Elt, used)
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "any"
		}
	}

	// Fallback for func, chan, struct, non-empty interface literals: collect qualifiers conservatively.
	ast.Inspect(expr, func(n ast.Node) bool {
		if se, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := se.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}

		return true
	})

	return strings.ReplaceAll(types.ExprString(expr), "interface{}", "any")
}

func getSupportedTemplates() []string {
	return slices.Sorted(maps.Keys(templateMap))
}
