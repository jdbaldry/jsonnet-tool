package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
  %s repl
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
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

// repl can be used for interactive evaluation of Jsonnet.
type repl struct {
	in     *bufio.Scanner
	out    io.Writer
	err    io.Writer
	locals []string
	vm     *jsonnet.VM
}

func (r *repl) read() string {
	r.in.Scan()
	input := r.in.Text()

	if err := r.in.Err(); err != nil {
		io.WriteString(r.err, fmt.Sprintf("Invalid input: %s", err))
	}
	return input
}

func (r *repl) eval(input string) (string, error) {
	if input == "help" || input == "?" {
		return `  Enter exit or quit to exit the repl.
  Enter locals to see the local variables.
  Enter ? or help to print this help again.
`, nil
	}
	if strings.HasPrefix(input, "locals") {
		if len(r.locals) == 0 {
			return "", nil
		}
		return strings.Join(append(r.locals, ";\n"), ";\n"), nil
	}
	if strings.HasPrefix(input, "local") {
		r.locals = append(r.locals, strings.Trim(input, ";"))
		return "", nil
	}
	result, err := r.vm.EvaluateAnonymousSnippet("repl", strings.Join(append(r.locals, input), ";"))
	if err != nil {
		return "", err
	}
	return result, nil
}

// newREPL produces a REPL.
func newREPL(in io.Reader, out io.Writer, err io.Writer) repl {
	return repl{in: bufio.NewScanner(in), out: out, err: err, locals: []string{}, vm: makeVM()}
}

type LocationRange struct {
	FileName string
	Begin    ast.Location
	End      ast.Location
}

// uncons returns the head of the slice and the tail of the slice.
func uncons(args []string) (string, []string) {
	if len(args) == 0 {
		return "", args
	}
	if len(args) == 1 {
		return args[0], []string{}
	}
	return args[0], args[1:]
}

func main() {
	args := os.Args
	if len(args) < 2 {
		help(os.Stderr)
		os.Exit(1)
	}

	_, args = uncons(args)
	command, args = uncons(args)

	switch command {
	case "dot":
		if len(args) != 1 {
			help(os.Stderr)
			os.Exit(1)
		}
		file, _ := uncons(args)
		vm := makeVM()
		root, _, err := vm.ImportAST("", file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to produce AST for file %s: %v\n", file, err)
			os.Exit(1)
		}
		out, err := dot(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error producing DOT from AST: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(out)

	case "expand":
		if len(args) != 1 {
			help(os.Stderr)
			os.Exit(1)
		}
		file, _ := uncons(args)
		vm := makeVM()
		root, _, err := vm.ImportAST("", file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to produce AST for file %s: %v\n", file, err)
			os.Exit(1)
		}
		out, err := vm.Expand(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error expanding AST: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(out)

	case "layers":
		if len(args) != 1 {
			help(os.Stderr)
			os.Exit(1)
		}
		file, _ := uncons(args)
		vm := makeVM()
		root, _, err := vm.ImportAST("", file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to produce AST for file %s: %v\n", file, err)
			os.Exit(1)
		}
		layers, err := findLayers(vm, root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing layers for file %s: %v\n", file, err)
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
		if len(args) != 1 {
			help(os.Stderr)
			os.Exit(1)
		}
		file, _ := uncons(args)
		vm := makeVM()
		imports, err := vm.FindDependencies("", []string{file})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to find imports for file %s: %v\n", file, err)
			os.Exit(1)
		}
		b, err := json.MarshalIndent(imports, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal to JSON: %v\n", err)
			os.Exit(1)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte{'\n'})

	case "repl":
		const prompt = "repl> "

		repl := newREPL(os.Stdin, os.Stdout, os.Stderr)

		// read
		_, err := io.WriteString(repl.out, prompt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to REPL out: %v\n", err)
			os.Exit(1)
		}
		input := repl.read()

		for {
			if input == "exit" || input == "quit" {
				_, err = io.WriteString(repl.out, "bye!\n")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error writing to REPL out: %v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}

			// eval
			result, err := repl.eval(input)
			if err != nil {
				_, err = io.WriteString(repl.err, err.Error())
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error writing to REPL err: %v\n", err)
					os.Exit(1)
				}
			}

			// print
			_, err = io.WriteString(repl.out, result)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to REPL out: %v\n", err)
				os.Exit(1)
			}

			// loop
			_, err = io.WriteString(repl.out, prompt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to REPL out: %v\n", err)
				os.Exit(1)
			}
			input = repl.read()
		}

	case "symbols":
		if len(args) != 1 {
			help(os.Stderr)
			os.Exit(1)
		}
		file, _ := uncons(args)
		vm := makeVM()
		root, _, err := vm.ImportAST("", file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to produce AST for file %s: %v\n", file, err)
			os.Exit(1)
		}
		symbols, err := findSymbols(&root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing symbols for file %s: %v\n", file, err)
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
