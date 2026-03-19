package metadata

import (
	"fmt"
	"unicode"

	"github.com/go-softwarelab/common/pkg/to"
)

// Package gathers the details about original package and the target package for generated code.
type Package struct {
	Name             string
	OriginalName     string
	OriginalFullName string
	IsSamePackage    bool
	typePrinter      func(string) string
}

// NewPackage creates a new Package instance.
func NewPackage(name, originalName, originalFullName string, isSamePackage bool) *Package {
	pkg := &Package{
		Name:             name,
		OriginalName:     originalName,
		OriginalFullName: originalFullName,
		IsSamePackage:    isSamePackage,
	}

	typePrinter := to.IfThen(isSamePackage, pkg.printForSamePackage).ElseThen(pkg.printForExternal)
	pkg.typePrinter = typePrinter

	return pkg
}

// OriginalPkgImportStatement returns the import statement for the original package if the target package is different from the original package.
func (pkg *Package) OriginalPkgImportStatement() string {
	if pkg.IsSamePackage {
		return ""
	}
	return fmt.Sprintf(`import "%s"`, pkg.OriginalFullName)
}

// PrintType returns the formatted type name with the package prefix if it's external or without it.
func (pkg *Package) PrintType(typeName string) string {
	return pkg.typePrinter(typeName)
}

func (pkg *Package) printForSamePackage(typeName string) string {
	return typeName
}

func (pkg *Package) printForExternal(typeName string) string {
	if typeName == "" {
		return typeName
	}

	star := ""

	if typeName[0] == '*' {
		star = "*"
		typeName = typeName[1:]
	}
	var result string
	if unicode.IsUpper(rune(typeName[0])) {
		result = fmt.Sprintf("%s%s.%s", star, pkg.OriginalName, typeName)
	} else {
		result = star + typeName
	}

	return result
}
