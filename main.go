package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/google/go-jsonnet"
)

var (
	inputFile string
)

func main() {
	flag.StringVar(&inputFile, "input", "", "input Jsonnet file")
	vm := jsonnet.MakeVM()
	out, _, err := vm.ImportAST(inputFile, inputFile)
	if err != nil {
		fmt.Printf("unable to convert snippet to AST: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
}
