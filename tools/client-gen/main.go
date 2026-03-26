//go:build gen

package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bsv-blockchain/go-wallet-toolbox/tools/client-gen/extractor"
	"github.com/bsv-blockchain/go-wallet-toolbox/tools/client-gen/generator"
	"github.com/bsv-blockchain/go-wallet-toolbox/tools/client-gen/metadata"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Parse command line flags
	outputFile := flag.String("out", "client_gen.go", "Output file (default: stdout)")

	workingDir := flag.String("cwd", cwd, "Current working directory")

	templateFile := flag.String("tmpl", "client.tpl", "Template file name in the templates directory (default: client.tpl)")

	skipMethodsFlag := flag.String("skip-methods", "", "Comma-separated list of methods to skip (default: none)")
	flag.Parse()

	fmt.Println("Running from directory:", *workingDir)

	if *outputFile == "" {
		log.Fatalf("-out cannot be empty")
	}

	dir := *workingDir
	targetFile := filepath.Join(dir, *outputFile)

	isTheSameDir := dir == filepath.Dir(targetFile)

	// Get the file name from the environment (set by go:generate)
	fileName := os.Getenv("GOFILE")
	if fileName == "" {
		log.Fatal("GOFILE environment variable not set. Are you running this through go:generate?")
	}

	packageName := os.Getenv("GOPACKAGE")
	if packageName == "" {
		log.Fatal("GOPACKAGE environment variable not set. Are you running this through go:generate?")
	}

	fullPackageName := getFullPackageName(dir)

	// Combine the directory and file name
	filePath := filepath.Join(dir, fileName)

	log.Printf("Analyzing %s (file://%s) from package %s (full: %s)\n", fileName, filePath, packageName, fullPackageName)

	// Parse the file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		log.Fatalf("Failed to parse file %s: %v", filePath, err)
	}

	// Extract interface information
	interfaces := extractor.ExtractInterfaces(fset, file)

	targetPackageName := packageName
	if !isTheSameDir {
		targetPackageName = filepath.Base(filepath.Dir(targetFile))
	}

	pkg := metadata.NewPackage(targetPackageName, packageName, fullPackageName, isTheSameDir)

	data := generator.Data(pkg, interfaces, strings.Split(*skipMethodsFlag, ","))

	output := generator.Generate(data, *templateFile)

	// Write the output
	log.Printf("Writing to file://%s \n", targetFile)
	//nolint:gosec
	if err := os.WriteFile(targetFile, output, 0o644); err != nil {
		log.Fatalf("Failed to write output to file: %v", err)
	}
}

func getFullPackageName(dir string) string {
	// Walk up directories to find go.mod
	modDir := dir
	for {
		if _, err := os.Stat(filepath.Join(modDir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(modDir)
		if parent == modDir {
			log.Fatalf("go.mod not found")
		}
		modDir = parent
	}

	// Read go.mod to extract module path
	//nolint:gosec
	modContent, err := os.ReadFile(filepath.Join(modDir, "go.mod"))
	if err != nil {
		log.Fatalf("failed to read go.mod: %v", err)
	}

	// Extract module name from go.mod
	modLines := strings.Split(string(modContent), "\n")
	var modulePath string
	for _, line := range modLines {
		if strings.HasPrefix(strings.TrimSpace(line), "module ") {
			modulePath = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "module "))
			break
		}
	}

	if modulePath == "" {
		log.Fatalf("module path not found in go.mod")
	}

	// Get the relative path from the module root to the package directory
	relPath, err := filepath.Rel(modDir, dir)
	if err != nil {
		log.Fatalf("failed to get relative path: %v", err)
	}

	// For root package, just return the module path
	if relPath == "." {
		return modulePath
	}

	// Otherwise, join the module path and the relative path
	p := path.Join(modulePath, relPath)
	return strings.ReplaceAll(p, "\\", "/")
}
