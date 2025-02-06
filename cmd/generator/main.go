package main

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/spf13/pflag"
)

//go:embed templates/otel_proxy.tmpl
var otelProxyTmpl string

// 定義模板類型和對應的模板內容
var templateMap = map[string]string{
	"otel": otelProxyTmpl,
}

type Method struct {
	Name       string
	Params     string
	Results    []string // 改為 slice 存儲所有返回值
	CallParams string
	HasError   bool
}

type TemplateData struct {
	PackageName     string
	ProxyName       string
	InterfaceName   string
	TracerName      string
	Methods         []Method
	Imports         []string
	InterfacePrefix string
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
	methods, imports, interfacePrefix, err := parseInterfaceMethods(sourceFile, outputFile, interfaceName)
	if err != nil {
		log.Fatalf("Failed to parse interface: %v", err)
	}

	// 構造模板數據
	data := TemplateData{
		PackageName:     packageName,
		ProxyName:       interfaceName + "Proxy",
		InterfaceName:   interfaceName,
		TracerName:      tracerName,
		Methods:         methods,
		Imports:         imports,
		InterfacePrefix: interfacePrefix,
	}

	// 生成代碼
	err = generateCode(templateFile, outputFile, data)
	if err != nil {
		log.Fatalf("Failed to generate code: %v", err)
	}

	fmt.Printf("Proxy for interface '%s' generated at '%s' using template '%s'\n", interfaceName, outputFile, templateType)
}

func parseInterfaceMethods(sourceFile, outputFile, interfaceName string) ([]Method, []string, string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, sourceFile, nil, parser.AllErrors)
	if err != nil {
		return nil, nil, "", err
	}

	// 收集 imports
	imports := []string{}

	// 獲取當前工作目錄
	workDir, err := os.Getwd()
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get working directory: %v", err)
	}

	// 將相對路徑轉換為絕對路徑
	absSourcePath, err := filepath.Abs(filepath.Join(workDir, sourceFile))
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get absolute source path: %v", err)
	}

	absOutputPath, err := filepath.Abs(filepath.Join(workDir, outputFile))
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get absolute output path: %v", err)
	}

	// 比較源文件和輸出文件的路徑
	sourcePath := filepath.Dir(absSourcePath)
	outputPath := filepath.Dir(absOutputPath)

	// 決定接口前綴
	interfacePrefix := ""
	if sourcePath != outputPath {
		interfacePrefix = "repository."
	}

	// 從源文件路徑中提取包路徑
	// 假設項目結構是 server-template/internal/...
	idx := strings.Index(sourcePath, "internal")
	if idx != -1 {
		packagePath := sourcePath[idx:]
		importPath := fmt.Sprintf(`"server-template/%s"`, packagePath)
		imports = append(imports, importPath)
	}

	// 收集其他 imports
	for _, imp := range node.Imports {
		imports = append(imports, imp.Path.Value)
	}

	// 對 imports 進行排序
	sort.Slice(imports, func(i, j int) bool {
		// 移除引號進行比較
		iStr := strings.Trim(imports[i], `"`)
		jStr := strings.Trim(imports[j], `"`)

		// 先比較大小寫（大寫優先）
		iHasUpper := strings.ToLower(iStr) != iStr
		jHasUpper := strings.ToLower(jStr) != jStr
		if iHasUpper != jHasUpper {
			return iHasUpper
		}

		// 然後按字母順序排序
		return iStr < jStr
	})

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

	return methods, imports, interfacePrefix, nil
}

func formatParams(fields *ast.FieldList) (string, string) {
	if fields == nil {
		return "", ""
	}

	var paramParts []string
	var callParts []string

	for _, f := range fields.List {
		t := formatType(f.Type)
		if len(f.Names) > 0 {
			for _, name := range f.Names {
				paramParts = append(paramParts, name.Name+" "+t)
				callParts = append(callParts, name.Name)
			}
		} else {
			paramParts = append(paramParts, t)
		}
	}

	return strings.Join(paramParts, ", "), strings.Join(callParts, ", ")
}

func formatResults(fields *ast.FieldList) ([]string, bool) {
	if fields == nil {
		return nil, false
	}

	var results []string
	hasError := false

	for _, f := range fields.List {
		t := formatType(f.Type)
		if t == "error" {
			hasError = true
		}
		results = append(results, t)
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
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", formatType(t.Key), formatType(t.Value))
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return fmt.Sprintf("/* unsupported type %T */", expr)
	}
}

func generateCode(templatePath, outputFile string, data TemplateData) error {
	funcMap := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
	}

	tmpl, err := template.New("proxy").Funcs(funcMap).Parse(templatePath)
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
