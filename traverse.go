package main

import (
	"fmt"

	"github.com/google/go-jsonnet/ast"
	"github.com/jdbaldry/jsonnet-tool/internal/parser"
)

// nop performs no operation on the AST node.
func nop(_ *ast.Node) error { return nil }

// traverse can be used to perform depth-first pre-order, in-order, or post-order
// traversal of the Jsonnet AST.
func traverse(root ast.Node, pre, in, post func(node *ast.Node) error) error {
	if err := pre(&root); err != nil {
		return fmt.Errorf("pre error: %w", err)
	}

	children := parser.Children(root)

	if len(children) == 0 {
		if err := in(&root); err != nil {
			return fmt.Errorf("in error: %w", err)
		}
		if err := post(&root); err != nil {
			return fmt.Errorf("post error: %w", err)
		}
		return nil
	}

	last := len(children) - 1
	for i := 0; i <= last-1; i++ {
		if err := traverse(children[i], pre, in, post); err != nil {
			return err
		}
	}

	if err := in(&root); err != nil {
		return fmt.Errorf("in error: %w", err)
	}

	if err := traverse(children[last], pre, in, post); err != nil {
		return err
	}

	if err := post(&root); err != nil {
		return fmt.Errorf("post error: %w", err)
	}

	return nil
}
