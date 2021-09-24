package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

var (
	command string
	errExit = errors.New("exit")
)

// help writes help text.
// If no writer is provided, it writes to stderr.
func help(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, `A tool for working with Jsonnet files.

Produce a .dot diagram of the Jsonnet AST for <file>:
  $ %s dot <file>
Evaluate Jsonnet using the jsonnet-tool interpreter:
  $ %s eval
Produce a JSON array of the layers of object evaluations for <file>:
  $ %s layers <file>
List the imports for <file>:
  $ %s imports <file>
List the referenceable symbols in <file>:
  $ %s symbols <file>
Run a Jsonnet REPL:
  $ %s repl
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
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
	help   string
	locals []string
	vm     *jsonnet.VM
}

func (r *repl) read() (string, error) {
	r.in.Scan()
	return r.in.Text(), r.in.Err()
}

// eval evaluates the input string.
// It expects the string to be trimmed of preceding whitespace.
// '\h' prints a help message.
// '\?' is an alias for \h.
// '\q' quits the REPL.
// '\l' prints a list of namespace variables. TODO: implement.
// '\l binds+=bind (COMMA binds+=bind)* SEMI_COLON' defines new variable bindings. TODO: implement.
// '\d i' removes the ith local variable binding. TODO: implement.
// '\f <file>' writes something? to a file. TODO: What should it write? Namespace bindings and... TODO: implement.
// '\n' prints a list of namespaces. TODO: implement.
// '\n i' switches to the ith namespace. TODO: implement.
// Anything else is evaluated as Jsonnet input.
func (r *repl) eval(input string) (string, error) {
	if len(input) == 0 {
		return input, errors.New("no input string provided")
	}
	switch input[0] {
	case '\\':
		if len(input) != 2 {
			return "", fmt.Errorf("expected command such as \\h, got %s", input)
		}
		switch input[1] {
		case 'h', '?':
			return r.help, nil
		case 'q':
			return "bye!\n", errExit
		default:
			return "", fmt.Errorf("unknown command %s", input)
		}
	default:
		result, err := r.vm.EvaluateAnonymousSnippet("repl", strings.Join(append(r.locals, input), ";"))
		if err != nil {
			return "", err
		}
		return result, nil
	}
}

// newREPL produces a REPL.
func newREPL(in io.Reader, out io.Writer, err io.Writer) repl {
	return repl{
		in:  bufio.NewScanner(in),
		out: out,
		err: err,
		help: `A Jsonnet REPL.

\h prints this help message.
\q quits the REPL.

Anything else is evaluated as Jsonnet.
`,
		locals: []string{},
		vm:     makeVM()}
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

	case "--help", "-h":
		help(os.Stdout)
		os.Exit(0)

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

	case "eval":
		if len(args) != 1 {
			help(os.Stderr)
			os.Exit(1)
		}
		file, _ := uncons(args)
		json, err := makeVM().EvaluateFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error evaluating Jsonnet for file %s:\n%v\n", file, err)
			os.Exit(1)
		}
		fmt.Print(json)

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

	case "repl":
		const prompt = "repl> "
		repl := newREPL(os.Stdin, os.Stdout, os.Stderr)

		// read
		fmt.Fprint(repl.out, repl.help)
		fmt.Fprint(repl.out, prompt)
		input, err := repl.read()
		if err != nil {
			fmt.Fprintf(repl.err, "Error reading input: %v\n", err)
			os.Exit(1)
		}

		for {
			// eval
			result, err := repl.eval(input)
			if err != nil {
				if err == errExit {
					fmt.Fprint(repl.out, result)
					os.Exit(0)
				}
				fmt.Fprintf(repl.out, "Evaluation error: %v\n", err)
			}

			// print
			fmt.Fprint(repl.out, result)

			// loop
			fmt.Fprint(repl.out, prompt)
			input, err = repl.read()
			if err != nil {
				fmt.Fprintf(repl.err, "Error reading input: %v\n", err)
			}
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
