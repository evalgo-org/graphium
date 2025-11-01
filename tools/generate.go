package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Model struct {
	Name   string
	Type   string
	Fields []Field
}

type Field struct {
	Name       string
	Type       string
	JSONLDTag  string
	CouchTag   string
	IsIndexed  bool
	IsRelation bool
}

func main() {
	models := parseModels(".")
	fmt.Printf("Found %d models\n", len(models))

	_ = os.MkdirAll("../generated", 0755)

	generatePlaceholder(models)

	fmt.Println("✓ Generated placeholder code")
}

func parseModels(dir string) []Model {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	var models []Model

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				typeSpec, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}

				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					return true
				}

				model := Model{
					Name: typeSpec.Name.Name,
					Type: typeSpec.Name.Name,
				}

				for _, field := range structType.Fields.List {
					if len(field.Names) == 0 {
						continue
					}

					f := Field{
						Name: field.Names[0].Name,
					}

					if field.Tag != nil {
						tag := field.Tag.Value
						f.JSONLDTag = extractTag(tag, "jsonld")
						f.CouchTag = extractTag(tag, "couchdb")
						f.IsIndexed = strings.Contains(f.CouchTag, "index")
						f.IsRelation = strings.Contains(f.CouchTag, "relation")
					}

					model.Fields = append(model.Fields, f)
				}

				if len(model.Fields) > 0 {
					models = append(models, model)
				}

				return true
			})
		}
	}

	return models
}

func extractTag(tag, key string) string {
	tag = strings.Trim(tag, "`")
	for _, part := range strings.Fields(tag) {
		if strings.HasPrefix(part, key+":") {
			value := strings.TrimPrefix(part, key+":")
			value = strings.Trim(value, `"`)
			return value
		}
	}
	return ""
}

func generatePlaceholder(models []Model) {
	content := `// Auto-generated code
package generated

// Placeholder for generated API
func Hello() string {
	return "Graphium Generated Code"
}
`

	filename := filepath.Join("../generated", "api.go")
	_ = os.WriteFile(filename, []byte(content), 0644)
}
