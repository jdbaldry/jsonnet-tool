// Package toolutils includes several utilities handy for use in code analysis tools
package toolutils

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/internal/parser"
)

// Children returns all children of a node. It supports ASTs before and after desugaring.
func Children(node ast.Node) []ast.Node {
	return parser.Children(node)
}

// SnippetToRawAST converts a Jsonnet code snippet to an AST (without any transformations).
// Any fodder after the final token is returned as well.
func SnippetToRawAST(diagnosticFilename ast.DiagnosticFileName, importedFilename, snippet string) (ast.Node, ast.Fodder, error) {
	return parser.SnippetToRawAST(diagnosticFilename, importedFilename, snippet)
}
