package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/formatter"

	"github.com/grafana/tanka/pkg/jsonnet/native"
)

var (
	command string
	errExit = errors.New("exit")
)

// scanDoubleSemiColon is a split function for a Scanner that returns each string of text
// separated by two semicolons ";;".
func scanDoubleSemiColon(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Skip leading spaces.
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !unicode.IsSpace(r) {
			break
		}
	}
	// Scan until two semicolons are encountered.
	var prev rune
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if r == ';' && prev == ';' {
			return i + 2*width, data[start : i-1], nil
		}
		prev = r
	}
	// If we're at EOF, we have a final, non-empty, non-terminated string of text.
	if atEOF && len(data) > start {
		return len(data), data[start:], nil
	}
	// Request more data.
	return start, nil, nil
}

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
  $ %s eval <file>

Produce an expanded Jsonnet representation:
  $ %s expand <file>

Produce a JSON array of the layers of object evaluations for <file>:
  $ %s layers <file>

List the imports for <file>:
  $ %s imports <file>

List the referenceable symbols in <file>:
  $ %s symbols <file>

Run a Jsonnet REPL:
  $ %s repl
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

// makeVM creates a Jsonnet VM configured to import from the Jpaths specified in the
// JSONNET_PATH environment variable.
// TODO: this should support -J flags too.
func makeVM() *jsonnet.VM {
	vm := jsonnet.MakeVM()
	importer := &jsonnet.FileImporter{JPaths: filepath.SplitList(os.Getenv("JSONNET_PATH"))}
	vm.Importer(importer)

	for _, fn := range native.Funcs() {
		vm.NativeFunction(fn)
	}

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
	// in is where the REPL reads input from.
	in *bufio.Scanner
	// evalFile is where the REPL will write out evaluations partitioned by namespace index.
	evalFile []string
	// namespaceFile is where the REPL will write out the current namespace partitioned by namespace index.
	namespaceFile []string
	// help is the REPL help text.
	help string
	// preExprs are a expressions partitioned by namespace index and prepended to evaluation.
	preExprs [][]string
	// ns is the index of the current namespace.
	ns int
	// vm performs the Jsonnet evaluations.
	vm *jsonnet.VM
}

// prompt returns the REPL prompt.
func (r *repl) prompt() string { return fmt.Sprintf("repl [%d]> ", r.ns) }

// read reads a line from the repl input.
func (r *repl) read() (string, error) {
	r.in.Scan()
	return r.in.Text(), r.in.Err()
}

// eval evaluates the input string.
// It expects the string to be trimmed of preceding whitespace.
// See the repl.help for behaviors.
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
			re := regexp.MustCompile(`^(?s)\\d\s+([0-9]+)$`)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 2 {
				return "", fmt.Errorf("invalid delete command syntax. Wanted \\d INDEX")
			}
			i, err := strconv.Atoi(matches[1])
			if err != nil {
				return "", fmt.Errorf("invalid delete command index.")
			}
			if i < 0 || i > len(r.preExprs[r.ns])-1 {
				return "", fmt.Errorf("delete command index out of range")
			}
			r.preExprs[r.ns] = append(r.preExprs[r.ns][:i], r.preExprs[r.ns][i+1:]...)
			return "", nil
		case 'f':
			re := regexp.MustCompile(`^(?s)\\f\s+(.+)$`)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 2 {
				return "", fmt.Errorf("invalid file command syntax. Wanted \\f FILE")
			}
			path, err := filepath.Abs(matches[1])
			if err != nil {
				return "", fmt.Errorf("unable to determine path to file: %w", err)
			}
			r.evalFile[r.ns] = path
			return fmt.Sprintf("Writing evaluations to file %s\n", r.evalFile[r.ns]), nil
		case 'h', '?':
			return r.help, nil
		case 'n':
			if len(input) == 2 {
				r.preExprs = append(r.preExprs, []string{})
				r.evalFile = append(r.evalFile, "")
				r.namespaceFile = append(r.namespaceFile, "")
				r.ns = len(r.preExprs) - 1
				return fmt.Sprintf("Switched to namespace %d\n", r.ns), nil
			}
			re := regexp.MustCompile(`^(?s)\\n\s+([0-9]+)$`)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 2 {
				return "", fmt.Errorf("invalid namespace command syntax. Wanted \\n or \\n INDEX")
			}
			i, err := strconv.Atoi(matches[1])
			if err != nil {
				return "", fmt.Errorf("invalid namespace command index.")
			}
			if i < 0 || i > len(r.preExprs)-1 {
				return "", fmt.Errorf("namespace command index out of range")
			}
			r.ns = i
			builder := strings.Builder{}
			builder.WriteString(fmt.Sprintf("Switched to namespace %d\n", r.ns))
			if r.evalFile[r.ns] != "" {
				builder.WriteString(fmt.Sprintf("Writing evaluations to file %s\n", r.evalFile[r.ns]))
			}
			if r.namespaceFile[r.ns] != "" {
				builder.WriteString(fmt.Sprintf("Writing namespace to file %s\n", r.namespaceFile[r.ns]))
			}
			return builder.String(), nil
		case 'q':
			return "", errExit
		case 'v':
			re := regexp.MustCompile(`(?s)^\\v\s*(.*)$`)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 2 {
				return "", fmt.Errorf("invalid variable expression command syntax. Wanted \\v or \\v EXPR.\n")
			}
			if len(matches[1]) > 0 {
				r.preExprs[r.ns] = append(r.preExprs[r.ns], strings.Trim(strings.TrimPrefix(input, `\v`), " ;"))
				return "", nil
			}
			builder := strings.Builder{}
			for i, s := range r.preExprs[r.ns] {
				builder.WriteString(fmt.Sprintf("[%d] %s\n", i, s))
			}
			return builder.String(), nil
		case 'w':
			re := regexp.MustCompile(`(?s)^\\w\s+(.+)$`)
			matches := re.FindStringSubmatch(input)
			if len(matches) != 2 {
				return "", fmt.Errorf("invalid write command syntax. Wanted \\w file")
			}
			path, err := filepath.Abs(matches[1])
			if err != nil {
				return "", fmt.Errorf("unable to determine path to file: %w", err)
			}
			r.namespaceFile[r.ns] = path
			return fmt.Sprintf("Writing namespace to file %s\n", r.namespaceFile[r.ns]), nil
		default:
			return "", fmt.Errorf("unknown command %s", input)
		}
	default:
		builder := strings.Builder{}
		for _, s := range r.preExprs[r.ns] {
			builder.WriteString(fmt.Sprintf("%s;\n", s))
		}
		builder.WriteString(input)
		if r.namespaceFile[r.ns] != "" {
			err := ioutil.WriteFile(r.namespaceFile[r.ns], []byte(builder.String()), 0o644)
			if err != nil {
				return "", fmt.Errorf("unable to write namespace to file %s: %w", r.namespaceFile, err)
			}
		}
		result, err := r.vm.EvaluateAnonymousSnippet("repl", builder.String())
		if err != nil {
			return "", err
		}
		if r.evalFile[r.ns] != "" {
			err := ioutil.WriteFile(r.evalFile[r.ns], []byte(result), 0o644)
			if err != nil {
				return "", fmt.Errorf("unable to write evaluation to file %s: %w", r.evalFile, err)
			}
		}
		return result, nil
	}
}

// newREPL produces a REPL.
func newREPL(in io.Reader) repl {
	scanner := bufio.NewScanner(in)
	scanner.Split(scanDoubleSemiColon)
	return repl{
		in:            scanner,
		evalFile:      make([]string, 1),
		namespaceFile: make([]string, 1),
		help: `A Jsonnet REPL.

Commands and expressions should be terminated with two semicolons ';;'.
For example,
repl [0]> \v local bar = 'Hello, world!';;
repl [0]> bar;;
"Hello, world!"

\d i            removes the ith namespace variable expression (zero indexed).
\f FILE         writes subsequent evaluation of the current namespace to FILE.
\n              creates a new namespace.
\n i            switches to the ith namespace (zero indexed).
\h              prints this help message.
\q              quits the REPL.
\v              prints the namespace expressions.
\v EXPR         creates a new namespace EXPR that is prepended to evaluation.
\w FILE         writes the state of the current namespace to FILE.
Anything else is evaluated as Jsonnet.
`,
		preExprs: make([][]string, 1),
		ns:       0,
		vm:       makeVM(),
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
		body, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to read file %s: %v\n", file, err)
		}
		root, _, err := formatter.SnippetToRawAST(file, string(body))
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
			// The newline after the initial error allows this tools error
			// output to match the regexps used by flycheck (and probably
			// other editor error checkers).
			fmt.Fprintf(os.Stderr, "Error evaluating Jsonnet for file %s:\n%v\n", file, err)
			os.Exit(1)
		}
		fmt.Print(json)

	case "expand":
		if len(args) != 1 {
			help(os.Stderr)
			os.Exit(1)
		}
		file, _ := uncons(args)
		input, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", file, err)
			os.Exit(1)
		}
		_, _, err = formatter.SnippetToRawAST(file, string(input))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing AST for file %s: %v\n", file, err)
			os.Exit(1)
		}
		// output, err := makeVM().Expand(root, finalFodder)
		// if err != nil {
		// 	fmt.Fprintf(os.Stderr, "Error expanding file %s: %v\n", file, err)
		// 	os.Exit(1)
		// }
		// fmt.Print(output)

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
					fmt.Println("Bye!")
					os.Exit(0)
				}
				fmt.Printf("Evaluation error: %v\n", err)
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
		symbols, err := findSymbols(&root, []string{"$"})
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
