package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMethodInfo_HasAnnotation(t *testing.T) {
	tests := []struct {
		name           string
		methodInfo     MethodInfo
		annotationType string
		expected       bool
	}{
		{
			name: "has annotation",
			methodInfo: MethodInfo{
				Annotations: []string{"@test", "@another"},
			},
			annotationType: "@test",
			expected:       true,
		},
		{
			name: "has annotation with prefix",
			methodInfo: MethodInfo{
				Annotations: []string{"@testPrefix", "@another"},
			},
			annotationType: "@test",
			expected:       true,
		},
		{
			name: "does not have annotation",
			methodInfo: MethodInfo{
				Annotations: []string{"@another", "@something"},
			},
			annotationType: "@test",
			expected:       false,
		},
		{
			name: "empty annotations",
			methodInfo: MethodInfo{
				Annotations: []string{},
			},
			annotationType: "@test",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.methodInfo.HasAnnotation(tt.annotationType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArguments_SkipTypes(t *testing.T) {
	tests := []struct {
		name      string
		arguments Arguments
		skipTypes []string
		expected  Arguments
	}{
		{
			name: "skip one type",
			arguments: Arguments{
				{Name: "param1", Type: "string"},
				{Name: "param2", Type: "int"},
				{Name: "param3", Type: "bool"},
			},
			skipTypes: []string{"int"},
			expected: Arguments{
				{Name: "param1", Type: "string"},
				{Name: "param3", Type: "bool"},
			},
		},
		{
			name: "skip multiple types",
			arguments: Arguments{
				{Name: "param1", Type: "string"},
				{Name: "param2", Type: "int"},
				{Name: "param3", Type: "bool"},
			},
			skipTypes: []string{"int", "bool"},
			expected: Arguments{
				{Name: "param1", Type: "string"},
			},
		},
		{
			name: "skip non-existent type",
			arguments: Arguments{
				{Name: "param1", Type: "string"},
				{Name: "param2", Type: "int"},
			},
			skipTypes: []string{"float"},
			expected: Arguments{
				{Name: "param1", Type: "string"},
				{Name: "param2", Type: "int"},
			},
		},
		{
			name:      "empty arguments",
			arguments: Arguments{},
			skipTypes: []string{"int"},
			expected:  Arguments{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.arguments.SkipTypes(tt.skipTypes...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArguments_ArgumentOfType(t *testing.T) {
	tests := []struct {
		name      string
		arguments Arguments
		typeName  string
		expected  string
	}{
		{
			name: "type exists",
			arguments: Arguments{
				{Name: "param1", Type: "string"},
				{Name: "param2", Type: "int"},
			},
			typeName: "int",
			expected: "param2",
		},
		{
			name: "type does not exist",
			arguments: Arguments{
				{Name: "param1", Type: "string"},
				{Name: "param2", Type: "int"},
			},
			typeName: "bool",
			expected: "",
		},
		{
			name:      "empty arguments",
			arguments: Arguments{},
			typeName:  "int",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.arguments.ArgumentOfType(tt.typeName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResults_HasError(t *testing.T) {
	tests := []struct {
		name     string
		results  Results
		expected bool
	}{
		{
			name: "has error",
			results: Results{
				{Type: "string"},
				{Type: "error"},
			},
			expected: true,
		},
		{
			name: "does not have error",
			results: Results{
				{Type: "string"},
				{Type: "int"},
			},
			expected: false,
		},
		{
			name:     "empty results",
			results:  Results{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.results.HasError()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResults_ReturnError(t *testing.T) {
	tests := []struct {
		name       string
		results    Results
		errVarName string
		expected   string
	}{
		{
			name: "return error only",
			results: Results{
				{Type: "error"},
			},
			errVarName: "err",
			expected:   "return err",
		},
		{
			name: "return string and error",
			results: Results{
				{Type: "string"},
				{Type: "error"},
			},
			errVarName: "err",
			expected:   `return "", err`,
		},
		{
			name: "return bool, int and error",
			results: Results{
				{Type: "bool"},
				{Type: "int"},
				{Type: "error"},
			},
			errVarName: "err",
			expected:   "return false, 0, err",
		},
		{
			name: "return pointer type",
			results: Results{
				{Type: "*MyType"},
				{Type: "error"},
			},
			errVarName: "err",
			expected:   "return nil, err",
		},
		{
			name: "return custom type",
			results: Results{
				{Type: "MyType"},
				{Type: "error"},
			},
			errVarName: "err",
			expected:   "return MyType{}, err",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := NewPackage("test", "test", "test", true)
			result := tt.results.ReturnError(*pkg, tt.errVarName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPointerType(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		expected bool
	}{
		{
			name:     "is pointer",
			typeName: "*MyType",
			expected: true,
		},
		{
			name:     "is not pointer",
			typeName: "MyType",
			expected: false,
		},
		{
			name:     "empty string",
			typeName: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPointerType(tt.typeName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNumberType(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		expected bool
	}{
		{
			name:     "is int",
			typeName: "int",
			expected: true,
		},
		{
			name:     "is int64",
			typeName: "int64",
			expected: true,
		},
		{
			name:     "is uint",
			typeName: "uint",
			expected: true,
		},
		{
			name:     "is uint32",
			typeName: "uint32",
			expected: true,
		},
		{
			name:     "is float",
			typeName: "float64",
			expected: true,
		},
		{
			name:     "is not number",
			typeName: "string",
			expected: false,
		},
		{
			name:     "empty string",
			typeName: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumberType(tt.typeName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
