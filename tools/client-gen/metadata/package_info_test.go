package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPackage(t *testing.T) {
	tests := []struct {
		name             string
		pkgName          string
		originalName     string
		originalFullName string
		isSamePackage    bool
		expected         *Package
	}{
		{
			name:             "same package",
			pkgName:          "test",
			originalName:     "test",
			originalFullName: "github.com/example/test",
			isSamePackage:    true,
			expected: &Package{
				Name:             "test",
				OriginalName:     "test",
				OriginalFullName: "github.com/example/test",
				IsSamePackage:    true,
			},
		},
		{
			name:             "different package",
			pkgName:          "client",
			originalName:     "test",
			originalFullName: "github.com/example/test",
			isSamePackage:    false,
			expected: &Package{
				Name:             "client",
				OriginalName:     "test",
				OriginalFullName: "github.com/example/test",
				IsSamePackage:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewPackage(tt.pkgName, tt.originalName, tt.originalFullName, tt.isSamePackage)

			// We can't directly compare the typePrinter function, so we'll check the other fields
			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.OriginalName, result.OriginalName)
			assert.Equal(t, tt.expected.OriginalFullName, result.OriginalFullName)
			assert.Equal(t, tt.expected.IsSamePackage, result.IsSamePackage)
		})
	}
}

func TestPackage_OriginalPkgImportStatement(t *testing.T) {
	tests := []struct {
		name     string
		pkg      *Package
		expected string
	}{
		{
			name: "same package - no import needed",
			pkg: &Package{
				Name:             "test",
				OriginalName:     "test",
				OriginalFullName: "github.com/example/test",
				IsSamePackage:    true,
			},
			expected: "",
		},
		{
			name: "different package - import needed",
			pkg: &Package{
				Name:             "client",
				OriginalName:     "test",
				OriginalFullName: "github.com/example/test",
				IsSamePackage:    false,
			},
			expected: `import "github.com/example/test"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pkg.OriginalPkgImportStatement()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPackage_PrintType(t *testing.T) {
	tests := []struct {
		name     string
		pkg      *Package
		typeName string
		expected string
	}{
		{
			name:     "same package - regular type",
			pkg:      NewPackage("test", "test", "github.com/example/test", true),
			typeName: "MyType",
			expected: "MyType",
		},
		{
			name:     "same package - pointer type",
			pkg:      NewPackage("test", "test", "github.com/example/test", true),
			typeName: "*MyType",
			expected: "*MyType",
		},
		{
			name:     "same package - primitive type",
			pkg:      NewPackage("test", "test", "github.com/example/test", true),
			typeName: "string",
			expected: "string",
		},
		{
			name:     "different package - exported type",
			pkg:      NewPackage("client", "test", "github.com/example/test", false),
			typeName: "MyType",
			expected: "test.MyType",
		},
		{
			name:     "different package - pointer to exported type",
			pkg:      NewPackage("client", "test", "github.com/example/test", false),
			typeName: "*MyType",
			expected: "*test.MyType",
		},
		{
			name:     "different package - primitive type",
			pkg:      NewPackage("client", "test", "github.com/example/test", false),
			typeName: "string",
			expected: "string",
		},
		{
			name:     "different package - empty type",
			pkg:      NewPackage("client", "test", "github.com/example/test", false),
			typeName: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pkg.PrintType(tt.typeName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
