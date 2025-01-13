package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"text/template"

	"github.com/spf13/pflag"
)

// 定義模板類型和路徑的映射
var templateMap = map[string]string{
	"otel": "internal/templates/otel_proxy.tmpl",
}

type Method struct {
	Name       string
	Params     string
	Results    string
	CallParams string
	HasError   bool
}

type TemplateData struct {
	PackageName   string
	ProxyName     string
	InterfaceName string
	TracerName    string
	Methods       []Method
}

func main() {
	// 命令行參數
	var (
		sourceFile    string
		outputFile    string
		packageName   string
		interfaceName string
		tracerName    string
		templateType  string
	)

	// 定義參數
	pflag.StringVar(&sourceFile, "source", "", "Source file containing the interface")
	pflag.StringVar(&outputFile, "output", "proxy.gen.go", "Output file for the generated proxy")
	pflag.StringVar(&packageName, "package", "", "Package name for the generated proxy")
	pflag.StringVar(&interfaceName, "interface", "", "Interface to generate proxy for")
	pflag.StringVar(&tracerName, "tracer", "", "Tracer name for the generated proxy")
	pflag.StringVar(&templateType, "template", "", "Template type (e.g., otel ...)")

	// 解析參數
	pflag.Parse()

	// 校驗參數
	if sourceFile == "" || interfaceName == "" || templateType == "" {
		log.Fatal("source, interface, and template must be specified")
	}

	// 從模板映射中獲取模板文件路徑
	templateFile, ok := templateMap[templateType]
	if !ok {
		log.Fatalf("Unsupported template type: %s. Supported types: %v", templateType, getSupportedTemplates())
	}

	// 解析接口文件
	methods, err := parseInterfaceMethods(sourceFile, interfaceName)
	if err != nil {
		log.Fatalf("Failed to parse interface: %v", err)
	}

	// 構造模板數據
	data := TemplateData{
		PackageName:   packageName,
		ProxyName:     interfaceName + "Proxy",
		InterfaceName: interfaceName,
		TracerName:    tracerName,
		Methods:       methods,
	}

	// 生成代碼
	err = generateCode(templateFile, outputFile, data)
	if err != nil {
		log.Fatalf("Failed to generate code: %v", err)
	}

	fmt.Printf("Proxy for interface '%s' generated at '%s' using template '%s'\n", interfaceName, outputFile, templateType)
}

func parseInterfaceMethods(sourceFile, interfaceName string) ([]Method, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, sourceFile, nil, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	var methods []Method
	ast.Inspect(node, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != interfaceName {
			return true
		}

		it, ok := ts.Type.(*ast.InterfaceType)
		if !ok {
			return true
		}

		for _, m := range it.Methods.List {
			if len(m.Names) == 0 {
				continue
			}

			ft, ok := m.Type.(*ast.FuncType)
			if !ok {
				continue
			}

			params, callParams := formatParams(ft.Params)
			results, hasError := formatResults(ft.Results)

			methods = append(methods, Method{
				Name:       m.Names[0].Name,
				Params:     params,
				Results:    results,
				CallParams: callParams,
				HasError:   hasError,
			})
		}

		return false
	})

	return methods, nil
}

func formatParams(fields *ast.FieldList) (string, string) {
	if fields == nil {
		return "", ""
	}

	var params, callParams string
	for i, f := range fields.List {
		for j, name := range f.Names {
			if i > 0 || j > 0 {
				params += ", "
				callParams += ", "
			}
			params += fmt.Sprintf("%s %s", name, formatType(f.Type))
			callParams += name.Name
		}
	}
	return params, callParams
}

func formatResults(fields *ast.FieldList) (string, bool) {
	if fields == nil {
		return "", false
	}

	var results string
	hasError := false
	for i, f := range fields.List {
		if i > 0 {
			results += ", "
		}

		t := formatType(f.Type)
		results += t

		if t == "error" {
			hasError = true
		}
	}
	return results, hasError
}

func formatType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + formatType(t.X)
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", formatType(t.X), t.Sel.Name)
	case *ast.ArrayType:
		return "[]" + formatType(t.Elt)
	default:
		return ""
	}
}

func generateCode(templatePath, outputFile string, data TemplateData) error {
	// 讀取模板文件
	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	tmpl, err := template.New("proxy").Parse(string(tmplContent))
	if err != nil {
		return err
	}

	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

func getSupportedTemplates() []string {
	keys := make([]string, 0, len(templateMap))
	for key := range templateMap {
		keys = append(keys, key)
	}
	return keys
}
