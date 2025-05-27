package metadata

import (
	"slices"
	"strings"

	"github.com/go-softwarelab/common/pkg/seq"
	slicesx "github.com/go-softwarelab/common/pkg/slices"
)

// MethodInfo holds information about a method in an interface
type MethodInfo struct {
	Name        string    `json:"name"`
	Comments    []string  `json:"comment"`
	Annotations []string  `json:"annotations"`
	Arguments   Arguments `json:"arguments"`
	Results     Results   `json:"results"`
}

// HasAnnotation checks if the method contains an annotation comment.
// Annotations are method-level comments starting with "// @", for example: "// @Write".
func (m MethodInfo) HasAnnotation(annotationType string) bool {
	return seq.Exists(seq.FromSlice(m.Annotations), func(it string) bool {
		return strings.HasPrefix(it, annotationType)
	})
}

// ParamInfo holds information about a parameter
type ParamInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// TypeInfo holds information about a type
type TypeInfo struct {
	Type string `json:"type"`
}

// Arguments represents a slice of ParamInfo, typically used to define method or function parameter information.
type Arguments []ParamInfo

// SkipTypes filters out ParamInfo elements with matching types from the Arguments slice and returns a new Arguments slice.
func (a Arguments) SkipTypes(names ...string) Arguments {
	return slicesx.Filter(a, func(it ParamInfo) bool {
		return !slices.Contains(names, it.Type)
	})
}

// ArgumentOfType searches Arguments for a parameter of the specified type and returns its name or an empty string if not found.
func (a Arguments) ArgumentOfType(typeName string) string {
	found := seq.Find(seq.FromSlice(a), func(it ParamInfo) bool {
		return it.Type == typeName
	})

	return found.OrZeroValue().Name
}

// Results represents a collection of TypeInfo typically used to describe the results of a method or function.
type Results []TypeInfo

// HasError checks whether any TypeInfo in the Results slice has a Type field equal to "error".
func (r Results) HasError() bool {
	return seq.Exists(seq.FromSlice(r), func(it TypeInfo) bool {
		return it.Type == "error"
	})
}

// ReturnError generates a code for returning zero values and error from given variable name
func (r Results) ReturnError(pkg Package, errVarName string) string {
	returns := slicesx.Map(r, func(it TypeInfo) string {
		switch it.Type {
		case "error":
			return errVarName
		case "string":
			return `""`
		case "bool":
			return "false"
		}

		switch {
		case isNumberType(it.Type):
			return "0"
		case isPointerType(it.Type):
			return "nil"
		default:
			return pkg.PrintType(it.Type) + "{}"
		}
	})

	return "return " + strings.Join(returns, ", ")
}

func isPointerType(typeName string) bool {
	return strings.HasPrefix(typeName, "*")
}

func isNumberType(typeName string) bool {
	return strings.HasPrefix(typeName, "uint") || strings.HasPrefix(typeName, "int") || strings.HasPrefix(typeName, "float")
}
