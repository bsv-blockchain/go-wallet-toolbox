package extractor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"iter"
	"log"
	"strings"

	"github.com/go-softwarelab/common/pkg/seq"

	"github.com/bsv-blockchain/go-wallet-toolbox/tools/client-gen/metadata"
)

// InterfaceInfo holds information about an interface
type InterfaceInfo struct {
	Imports []Import              `json:"imports"`
	Name    string                `json:"name"`
	Methods []metadata.MethodInfo `json:"methods"`
}

// Import holds information about an import
type Import struct {
	Path  string // Full import path
	Alias string // Optional alias (may be empty)
}

func (i *Import) String() string {
	if i.Alias != "" {
		return fmt.Sprintf(`%s %s`, i.Alias, i.Path)
	}
	return i.Path
}

// ExtractInterfaces extracts interface information from the given file
func ExtractInterfaces(fset *token.FileSet, file *ast.File) []InterfaceInfo {
	var interfaces []InterfaceInfo

	// Build a map of package aliases to import paths
	importsFromFile := buildImportMap(file)

	// Inspect the AST and extract interfaces
	ast.Inspect(file, func(n ast.Node) bool {
		// Look for type declarations within GenDecl nodes
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			return true
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Check if it's an interface
			interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}

			// Create a new InterfaceInfo with empty imports
			iface := InterfaceInfo{
				Name:    typeSpec.Name.Name,
				Imports: []Import{},
			}

			// collect all to get only unique at the end
			allInterfaceImportsNames := seq.Of[string]()

			// Extract methods from the interface
			if interfaceType.Methods != nil {
				for _, method := range interfaceType.Methods.List {
					// Skip embedded interfaces
					if len(method.Names) == 0 {
						continue
					}

					methodInfo := metadata.MethodInfo{
						Name:        method.Names[0].Name,
						Annotations: make([]string, 0),
						Comments:    make([]string, 0),
					}

					for _, comment := range method.Doc.List {
						text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
						if strings.HasPrefix(text, "@") {
							methodInfo.Annotations = append(methodInfo.Annotations, text)
							continue
						}
						methodInfo.Comments = append(methodInfo.Comments, text)
					}

					// Get the function type
					funcType, ok := method.Type.(*ast.FuncType)
					if !ok {
						continue
					}

					// Extract arguments
					if funcType.Params != nil {
						for _, param := range funcType.Params.List {
							typeStr := typeToString(fset, param.Type)

							// Find imports used in this type
							imports := findImports(param.Type)
							allInterfaceImportsNames = seq.Concat(allInterfaceImportsNames, imports)

							// A single field can have multiple names (e.g., a, b int)
							if len(param.Names) > 0 {
								for _, name := range param.Names {
									paramInfo := metadata.ParamInfo{
										Name: name.Name,
										Type: typeStr,
									}
									methodInfo.Arguments = append(methodInfo.Arguments, paramInfo)
								}
							} else {
								// Unnamed parameter
								paramInfo := metadata.ParamInfo{
									Name: "",
									Type: typeStr,
								}
								methodInfo.Arguments = append(methodInfo.Arguments, paramInfo)
							}
						}
					}

					// Extract results
					if funcType.Results != nil {
						for _, result := range funcType.Results.List {
							typeStr := typeToString(fset, result.Type)

							// Find imports used in this type
							imports := findImports(result.Type)
							allInterfaceImportsNames = seq.Concat(allInterfaceImportsNames, imports)

							// A single result field can represent multiple results of the same type
							if len(result.Names) > 0 {
								for _, name := range result.Names {
									resultInfo := metadata.TypeInfo{
										Type: fmt.Sprintf("%s %s", name.Name, typeStr),
									}
									methodInfo.Results = append(methodInfo.Results, resultInfo)
								}
							} else {
								// Unnamed result
								resultInfo := metadata.TypeInfo{
									Type: typeStr,
								}
								methodInfo.Results = append(methodInfo.Results, resultInfo)
							}
						}
					}

					iface.Methods = append(iface.Methods, methodInfo)
				}
			}

			allInterfaceImportsNames = seq.Uniq(allInterfaceImportsNames)

			iface.Imports = seq.Collect(seq.Map(allInterfaceImportsNames, func(name string) Import {
				return importsFromFile[name]
			}))

			interfaces = append(interfaces, iface)
		}
		return true
	})

	return interfaces
}

// buildImportMap creates a map of package aliases to import paths
func buildImportMap(file *ast.File) map[string]Import {
	importMap := make(map[string]Import)

	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		var alias string
		var name string

		if imp.Name != nil {
			// Has explicit alias
			alias = imp.Name.Name
			name = alias
		} else {
			// Use the last segment of the import path as the package name
			segments := strings.Split(path, "/")
			name = segments[len(segments)-1]
		}

		importMap[name] = Import{
			Path:  imp.Path.Value,
			Alias: alias,
		}
	}

	return importMap
}

// findImports recursively examines an AST type expression to find imported types
func findImports(expr ast.Expr) (importNames iter.Seq[string]) {
	if expr == nil {
		return importNames
	}

	switch t := expr.(type) {
	case *ast.Ident:
		// This is a type like string
		return seq.Of[string]()

	case *ast.SelectorExpr:
		// This is a type like pkg.Type
		if ident, ok := t.X.(*ast.Ident); ok {
			return seq.Of(ident.Name)
		}

	case *ast.StarExpr:
		// Pointer type like *pkg.Type
		return findImports(t.X)

	case *ast.ArrayType:
		// Array/slice type like []pkg.Type
		return findImports(t.Elt)

	case *ast.MapType:
		// Map type like map[pkg.KeyType]pkg.ValueType
		return seq.Concat(
			findImports(t.Key),
			findImports(t.Value),
		)

	case *ast.ChanType:
		// Channel type like chan pkg.Type
		return findImports(t.Value)

	case *ast.FuncType:
		// Function type like func(pkg.Type) pkg.Type

		var params iter.Seq[*ast.Field]
		if t.Params != nil {
			params = seq.Concat(params, seq.FromSlice(t.Params.List))
		}
		if t.Results != nil {
			params = seq.Concat(params, seq.FromSlice(t.Results.List))
		}

		return seq.Flatten(seq.Map(params, func(param *ast.Field) iter.Seq[string] {
			return findImports(param.Type)
		}))

	default:
		log.Printf("[WARN] Not implemented support for imports for type %T used in interface", t)
	}
	return seq.Of[string]()
}

// typeToString converts an AST type to a string representation
func typeToString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	err := format.Node(&buf, fset, expr)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return strings.TrimSpace(buf.String())
}
