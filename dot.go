package main

import (
	"fmt"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/jdbaldry/jsonnet-tool/internal/parser"
)

// toString provides a reasonably concise string representation of the Jsonnet AST node.
// loc is useful as not all nodes have location information. For example, object fields have a location,
// but the LiteralString is the Name of a field does not.
func toString(node ast.Node, loc *ast.LocationRange) string {
	switch node := node.(type) {
	case *ast.Binary:
		return fmt.Sprintf("[%s] %p %T %s", loc, node, node, node.Op)
	case *ast.DesugaredObject:
		return fmt.Sprintf("[%s] %p %T", loc, node, node)
	case *ast.LiteralString:
		return fmt.Sprintf("[%s] %p %T %s", loc, node, node, node.Value)
	case *ast.Import:
		return fmt.Sprintf("[%s] %p %T", loc, node, node)
	case *ast.Var:
		return fmt.Sprintf("[%s] %p %T %s", loc, node, node, node.Id)
	default:
		return fmt.Sprintf("[%s] %p %T", loc, node, node)
	}
}

// dot produces a DOT language graph for the Jsonnet AST.
func dot(root ast.Node) (string, error) {
	builder := strings.Builder{}
	builder.WriteString("digraph {\n")
	err := traverse(root,
		nop,
		func(node *ast.Node) error {
			switch node := (*node).(type) {
			case *ast.DesugaredObject:
				for _, field := range node.Fields {
					builder.WriteString(fmt.Sprintf("  \"%s\"->\"%s\"\n",
						strings.ReplaceAll(toString(node, node.Loc()), `"`, `\"`),
						strings.ReplaceAll(toString(field.Name, &field.LocRange), `"`, `\"`)),
					)
					builder.WriteString(fmt.Sprintf("  \"%s\"->\"%s\"\n",
						strings.ReplaceAll(toString(field.Name, &field.LocRange), `"`, `\"`),
						strings.ReplaceAll(toString(field.Body, field.Body.Loc()), `"`, `\"`)),
					)
				}
				return nil
			default:
				for _, child := range parser.Children(node) {
					builder.WriteString(fmt.Sprintf("  \"%s\"->\"%s\"\n",
						strings.ReplaceAll(toString(node, node.Loc()), `"`, `\"`),
						strings.ReplaceAll(toString(child, child.Loc()), `"`, `\"`)),
					)
				}
				return nil
			}
		},
		nop,
	)
	builder.WriteString("}\n")
	return builder.String(), err
}
