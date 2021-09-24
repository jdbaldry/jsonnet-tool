package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// locals are a slice of local variable binding expressions that are prepended to Jsonnet evaluations.
type locals []string

// repl can be used for interactive evaluation of Jsonnet.
type repl struct {
	// in is where the REPL reads input from.
	in *bufio.Scanner
	// file is where the REPL will write out the current namespace on the next loop.
	file string
	// help is the REPL help text.
	help string
	// locals are a local variable expressions partitioned by namespace index.
	locals []locals
	// namespace is the index of the current namespace.
	namespace int
	// vm performs the Jsonnet evaluations.
	vm *jsonnet.VM
}

// prompt returns the REPL prompt.
func (r *repl) prompt() string { return fmt.Sprintf("repl [%d]> ", r.namespace) }

// read reads a line from the repl input.
func (r *repl) read() (string, error) {
	r.in.Scan()
	return r.in.Text(), r.in.Err()
}

// eval evaluates the input string.
// It expects the string to be trimmed of preceding whitespace.
// '\d i' removes the ith namespace variable binding (zero indexed).
// '\f file' writes output of next evaluation to a file. TODO: implement
// '\h' prints a help message.
// '\?' is an alias for \h.
// '\l' prints a list of namespace variables.
// '\l ID = EXPR' creates a new namespace variable.
// '\n' creates a new namespace.
// '\n i' switches to the ith namespace (zero indexed).
// '\w file' writes the namespace variables and next Jsonnet expression to file.
// Anything else is evaluated as Jsonnet input.
func (r *repl) eval(input string) (string, error) {
	if len(input) == 0 {
		return "", errExit
	}
	switch input[0] {
	case '\\':
		if len(input) < 2 {
			return r.help, fmt.Errorf("expected command such as \\h, got %s", input)
		}
		switch input[1] {
		case 'd':
			re := regexp.MustCompile(`^\\d\s+([0-9]+)$`)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 2 {
				return "", fmt.Errorf("invalid delete command syntax. Wanted \\d INDEX")
			}
			i, err := strconv.Atoi(matches[1])
			if err != nil {
				return "", fmt.Errorf("invalid delete command index.")
			}
			if i < 0 || i > len(r.locals[r.namespace])-1 {
				return "", fmt.Errorf("delete command index out of range")
			}
			r.locals[r.namespace] = append(r.locals[r.namespace][:i], r.locals[r.namespace][i+1:]...)
			return "", nil
		case 'h', '?':
			return r.help, nil
		case 'l':
			if len(input) == 2 {
				builder := strings.Builder{}
				for i, s := range r.locals[r.namespace] {
					builder.WriteString(fmt.Sprintf("[%d] local %s;\n", i, s))
				}
				return builder.String(), nil
			}

			re := regexp.MustCompile(`^\\l\s+([a-zA-Z_][a-zA-Z0-9_]*\s+=\s+[^;]+);?$`)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 2 {
				return "", fmt.Errorf("invalid local command syntax. Wanted \\l ID = EXPR")
			}
			r.locals[r.namespace] = append(r.locals[r.namespace], matches[1])
			return "", nil
		case 'n':
			if len(input) == 2 {
				r.locals = append(r.locals, []string{})
				r.namespace = len(r.locals) - 1
				return "", nil
			}
			re := regexp.MustCompile(`^\\n\s+([0-9]+)$`)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 2 {
				return "", fmt.Errorf("invalid namespace command syntax. Wanted \\n INDEX")
			}
			i, err := strconv.Atoi(matches[1])
			if err != nil {
				return "", fmt.Errorf("invalid namespace command index.")
			}
			if i < 0 || i > len(r.locals)-1 {
				return "", fmt.Errorf("namespace command index out of range")
			}
			r.namespace = i
			return "", nil
		case 'q':
			return "bye!\n", errExit
		case 'w':
			re := regexp.MustCompile(`^\\w\s+(.*)$`)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 2 {
				return "", fmt.Errorf("invalid write command syntax. Wanted \\w file")
			}
			path, err := filepath.Abs(matches[1])
			if err != nil {
				return "", fmt.Errorf("unable to determine path to file: %w", err)
			}
			r.file = path
			return fmt.Sprintf("The next REPL loop will write this namespace to file %s", r.file), nil
		default:
			return "", fmt.Errorf("unknown command %s", input)
		}
	default:
		builder := strings.Builder{}
		for _, s := range r.locals[r.namespace] {
			builder.WriteString(fmt.Sprintf("local %s;\n", s))
		}
		builder.WriteString(input)
		if r.file != "" {
			err := os.WriteFile(r.file, []byte(builder.String()), 0644)
			r.file = ""
			if err != nil {
				return "", err
			}
		}
		result, err := r.vm.EvaluateAnonymousSnippet("repl", builder.String())
		if err != nil {
			return "", err
		}
		return result, nil
	}
}

// newREPL produces a REPL.
func newREPL(in io.Reader) repl {
	return repl{
		in:   bufio.NewScanner(in),
		file: "",
		help: `A Jsonnet REPL.

\d i            removes the ith namespace variable binding (zero indexed).
\n              creates a new namespace.
\n i            switches to the ith namespace (zero indexed).
\h              prints this help message.
\l              prints the namespace variables.
\l ID = EXPR    adds a new namespace variable.
\q              quits the REPL.
\w file         writes the namespace variables and next Jsonnet expression to file.
Anything else is evaluated as Jsonnet.
`,
		locals:    make([]locals, 1),
		namespace: 0,
		vm:        makeVM(),
	}
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
		repl := newREPL(os.Stdin)

		// read
		fmt.Print(repl.help)
		fmt.Print(repl.prompt())
		input, err := repl.read()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			os.Exit(1)
		}

		for {
			// eval
			result, err := repl.eval(input)
			if err != nil {
				if err == errExit {
					os.Exit(0)
				}
				fmt.Fprintf(os.Stdout, "Evaluation error: %v\n", err)
			}

			// print
			fmt.Print(result)

			// loop
			fmt.Print(repl.prompt())
			input, err = repl.read()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
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
