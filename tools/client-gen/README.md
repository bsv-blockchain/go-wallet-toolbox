# Client-Gen: Code Generation Tool

Client-Gen is a code generation tool designed to automatically create client implementations from Go interface definitions.
It simplifies the process of creating client code by analyzing interface declarations and generating corresponding client implementations based on customizable templates.

## Purpose

The primary purpose of Client-Gen was to reduce boilerplate code when implementing storage interface.
By defining your service contract as a Go interface and adding appropriate annotations, you can automatically generate client code that adheres to that contract.

## Adding a New Template

To add a new template for code generation:

1. Create a new template file in the `tools/client-gen/generator/templates` directory with a `.tpl` extension
2. Design your template using Go's text/template syntax
3. Use the template with go:generate by adding a directive to your code:

```
//go:generate go run -tags gen ../../tools/client-gen/main.go -out path/to/output.go -tmpl your_template.tpl
```

### Template Parameters

The following information is available in the template context:

- `Package`: Package information
  - `Name`: Target package name
  - `OriginalName`: Original package name
  - `OriginalFullName`: Full package import path
  - `IsSamePackage`: Whether the target and original packages are the same
  - `PrintType(typeName)`: Function to properly format type names

- `Interfaces`: List of interface information
  - `Name`: Interface name
  - `Methods`: List of methods in the interface

- `Imports`: List of imports required by the generated code

### Method Information

Each method in an interface provides:

- `Name`: Method name
- `Comments`: Documentation comments
- `Annotations`: Special annotations (starting with `@`)
- `Arguments`: Method parameters
- `Results`: Return values

### Utility Functions

Templates can use these utility functions:

- `printType`: Formats type names correctly based on package context
- `contains`: Checks if a slice contains a value
- `coalesce`: Returns the first non-empty value from a list

### Annotations

Annotations are special comments that start with `@` and provide metadata about methods. For example:

```
// @Write
MethodName(ctx context.Context, param string) (result *Response, error)
```

In the example from `pkg/wdk/storage.interface.go`, annotations like `@Write` and `@Read` are used to indicate whether a method performs write or read operations.

You can access annotations in templates using the `Annotations` field of the `MethodInfo` struct, or use the `HasAnnotation` method to check for specific annotations:

```
{{if .HasAnnotation "@Write"}}
// This is a write operation
{{end}}
```

## Extending Functionality

To extend the Client-Gen tool:

1. Add new template functions in `generator/output.go`
2. Enhance interface extraction in `extractor/parser.go`
3. Add new metadata structures in the `metadata` package
4. Create new templates for different code generation needs

### Code Organization

The Client-Gen tool is organized into several packages:

- `extractor`: Responsible for parsing Go code and extracting interface information
- `generator`: Handles code generation using templates
- `metadata`: Defines data structures used by the generator

### Running Without go:generate

To run Client-Gen directly without using go:generate:

```bash
go run -tags gen tools/client-gen/main.go -out path/to/output.go -tmpl template.tpl
```

Required environment variables:

- `GOFILE`: The Go file containing the interface definition
- `GOPACKAGE`: The package of the Go file

You can set these manually:

```bash
GOFILE=storage.interface.go GOPACKAGE=wdk go run -tags gen tools/client-gen/main.go -out client_gen.go -tmpl client.tpl
```
