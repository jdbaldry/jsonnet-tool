package main

import (
	"fmt"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

// layer is an intermediate Jsonnet evaluation and its location.
type layer struct {
	Evaluation    string
	LocationRange LocationRange
}

// evaluatesToObject returns a boolean representing whether or not the evaluation of a Jsonnet
// node evaluates to a JSON object value.
// TODO: implement.
func evaluatesToObject(node *ast.Node) bool {
	return true
}

// findLayers returns intermediate layers of evaluation of the top level Jsonnet. The first layer in the slice is the final evaluation.
// Each subsequent layer steps through the binary merges of objects.
// For example: { a: 1 } + { a: 2 } would return layers:
// { "a": 2 }
// { "a": 1 }
func findLayers(vm *jsonnet.VM, root ast.Node) (layers []layer, err error) {
	final, err := vm.Evaluate(root)
	if err != nil {
		return layers, fmt.Errorf("error evaluating root Jsonnet: %w", err)
	}
	layers = append(layers, layer{
		Evaluation: final,
		LocationRange: LocationRange{
			FileName: root.Loc().FileName,
			Begin:    root.Loc().Begin,
			End:      root.Loc().End,
		},
	})

	// Perform a pre-order traversal of the AST, removing the RHS of any '+' binary operation performed on objects.
	err = traverse(root,
		func(node *ast.Node) error {
			switch i := (*node).(type) {
			case *ast.Binary:
				if i.Op == ast.BopPlus {
					if evaluatesToObject(&i.Right) {
						intermediate := layer{
							LocationRange: LocationRange{
								FileName: i.Left.Loc().FileName,
								Begin:    i.Left.Loc().Begin,
								End:      i.Left.Loc().End,
							},
						}
						i.Right = &ast.DesugaredObject{}
						intermediate.Evaluation, err = vm.Evaluate(root)
						// Not all errors are evaluation errors but for simplicity, this is ignored.
						if err != nil {
							intermediate.Evaluation = fmt.Sprintln(err)
						}
						layers = append(layers, intermediate)
					}
				}
			}
			return nil
		},
		nop,
		nop,
	)
	return
}
