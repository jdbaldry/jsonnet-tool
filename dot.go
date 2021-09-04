package main

import (
	"fmt"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/jdbaldry/jsonnet-tool/internal/parser"
)

// toString provides a reasonably concise string representation of the Jsonnet AST node.
func toString(n ast.Node) string {
	switch i := n.(type) {
	case *ast.Binary:
		return fmt.Sprintf("[%s] %p %T %s", i.Loc(), i, i, i.Op)
	case *ast.DesugaredObject:
		return fmt.Sprintf("[%s] %p %T", i.Loc(), i, i)
	case *ast.LiteralString:
		return fmt.Sprintf("%T %p %s", i, i, i.Value)
	case *ast.Import:
		return fmt.Sprintf("[%s] %p %T", i.Loc(), i, i)
	default:
		return fmt.Sprintf("[%s] %p %T", i.Loc(), i, i)
	}
}

// dot produces a DOT language graph for the Jsonnet AST.
func dot(root ast.Node) (string, error) {
	builder := strings.Builder{}
	builder.WriteString("digraph {\n")
	err := traverse(root,
		nop,
		func(node *ast.Node) error {
			for _, child := range parser.Children(*node) {
				builder.WriteString(fmt.Sprintf("  \"%s\"->\"%s\"\n",
					strings.ReplaceAll(toString(*node), `"`, `\"`),
					strings.ReplaceAll(toString(child), `"`, `\"`)),
				)
			}
			return nil
		},
		nop,
	)
	builder.WriteString("}\n")
	return builder.String(), err
}
