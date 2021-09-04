package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

var (
	command   string
	inputFile string
)

// help writes help text.
// If no writer is provided, it writes to stderr.
func help(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, `A tool for working with Jsonnet files.

Usage:
  %s dot <file>
  %s layers <file>
  %s imports <file>
  %s symbols <file>
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

// makeVM creates a Jsonnet VM configured to import from the Jpaths specified in the
// JSONNET_PATH environment variable.
// TODO: this should support -J flags too.
func makeVM() *jsonnet.VM {
	vm := jsonnet.MakeVM()
	importer := &jsonnet.FileImporter{JPaths: filepath.SplitList(os.Getenv("JSONNET_PATH"))}
	vm.Importer(importer)

	// Add in a `manifestYamlFromJson` native function which is used by a number of Jsonnet libraries.
	// I don't care for YAML so it actually outputs JSON.
	manifestYaml := &jsonnet.NativeFunction{
		Func: func(data []interface{}) (interface{}, error) {
			bytes, err := json.Marshal(data[0])
			if err != nil {
				return nil, err
			}
			return string(bytes), nil
		},
		Params: []ast.Identifier{"json"},
		Name:   "manifestYamlFromJson",
	}
	vm.NativeFunction(manifestYaml)
	return vm
}

type LocationRange struct {
	FileName string
	Begin    ast.Location
	End      ast.Location
}

func main() {
	if len(os.Args) != 3 {
		help(os.Stderr)
		os.Exit(1)
	}

	command = os.Args[1]
	inputFile = os.Args[2]

	switch command {
	case "dot":
		vm := makeVM()
		root, _, err := vm.ImportAST("", inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to produce AST for file %s: %v\n", inputFile, err)
			os.Exit(1)
		}
		out, err := dot(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error producing DOT from AST: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(out)

	case "expand":
		vm := makeVM()
		root, _, err := vm.ImportAST("", inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to produce AST for file %s: %v\n", inputFile, err)
			os.Exit(1)
		}
		out, err := expand(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding AST: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(out)

	case "layers":
		vm := makeVM()
		root, _, err := vm.ImportAST("", inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to produce AST for file %s: %v\n", inputFile, err)
			os.Exit(1)
		}
		layers, err := findLayers(vm, root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing layers for file %s: %v\n", inputFile, err)
			os.Exit(1)
		}
		b, err := json.MarshalIndent(layers, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal to JSON: %v\n", err)
			os.Exit(1)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte{'\n'})

	case "imports":
		vm := makeVM()
		imports, err := vm.FindDependencies("", []string{inputFile})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to find imports for file %s: %v\n", inputFile, err)
			os.Exit(1)
		}
		b, err := json.MarshalIndent(imports, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal to JSON: %v\n", err)
			os.Exit(1)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte{'\n'})

	case "symbols":
		vm := makeVM()
		root, _, err := vm.ImportAST("", inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to produce AST for file %s: %v\n", inputFile, err)
			os.Exit(1)
		}
		symbols, err := findSymbols(&root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing symbols for file %s: %v\n", inputFile, err)
			os.Exit(1)
		}
		b, err := json.MarshalIndent(symbols, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal to JSON: %v\n", err)
			os.Exit(1)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte{'\n'})

	default:
		fmt.Fprintf(os.Stderr, "Unrecognized command %s\n", command)
		help(os.Stderr)
		os.Exit(1)
	}
}
